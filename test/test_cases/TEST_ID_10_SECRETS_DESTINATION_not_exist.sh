#!/bin/bash
set -euxo pipefail

source $TEST_CASES_UTILS

create_secret_access_role

create_secret_access_role_binding

echo "Create test-env pod. SECRETS_DESTINATION is with invalid value 'incorrect_secrets'"
export SECRETS_DESTINATION_KEY_VALUE=" "
deploy_test_env

pod_name=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')

echo "Expecting secrets provider to fail with error 'CSPFK004E Environment variable 'SECRETS_DESTINATION' must be provided'"
wait_for_it 600 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK004E'"

