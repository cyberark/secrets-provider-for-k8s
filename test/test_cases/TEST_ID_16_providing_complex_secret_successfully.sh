#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

set_namespace "$CONJUR_NAMESPACE_NAME"
conjur_cli_pod=$(get_conjur_cli_pod_name)
secret_value="{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}"
$cli_with_timeout "exec $conjur_cli_pod -- conjur variable values add secrets/test_secret $secret_value"
set_namespace "$TEST_APP_NAMESPACE_NAME"

deploy_test_env

echo "Verifying pod test_env has environment variable 'TEST_SECRET' with value '$secret_value'"
pod_name=$(cli_get_pods_test_env | awk '{print $1}')
verify_secret_value_in_pod "$pod_name" TEST_SECRET "$secret_value"

# set the secret back to original one for the rest of the tests
set_namespace "$CONJUR_NAMESPACE_NAME"
$cli_with_timeout "exec $conjur_cli_pod -- conjur variable values add secrets/test_secret supersecret"
set_namespace "$TEST_APP_NAMESPACE_NAME"

