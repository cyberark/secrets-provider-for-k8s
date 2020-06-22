#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

export CONJUR_AUTHN_LOGIN="host/${APP_NAMESPACE_NAME}/*/*"

deploy_test_env

echo "Verifying pod test_env has environment variable 'TEST_SECRET' with value 'supersecret'"
pod_name=$(cli_get_pods_test_env | awk '{print $1}')
verify_secret_value_in_pod $pod_name TEST_SECRET supersecret
