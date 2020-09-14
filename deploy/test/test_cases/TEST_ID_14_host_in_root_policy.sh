#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

export CONJUR_AUTHN_LOGIN="host/${APP_NAMESPACE_NAME}/*/*"

deploy_init_env

echo "Verifying pod test_env has environment variable 'TEST_SECRET' with value 'supersecret'"
$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^1$'"
pod_name=$($cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers" | awk '{print $1}')

verify_secret_value_in_pod $pod_name TEST_SECRET supersecret
