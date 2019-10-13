#!/bin/bash
set -euxo pipefail

source $TEST_CASES_UTILS

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env with incorrect value for SECRETS_DESTINATION envrionment variable"
export SECRETS_DESTINATION_KEY_VALUE="          - name: SECRETS_DESTINATION"$'\n'"            value: SECRETS_DESTINATION_incorrect_value"
deploy_test_env

echo "Expecting secrets provider to fail with error 'CSPFK005E Provided incorrect value for environment variable SECRETS_DESTINATION'"
pod_name=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')
wait_for_it 30 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK005E'"

