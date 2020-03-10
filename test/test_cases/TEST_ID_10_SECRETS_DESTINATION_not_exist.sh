#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Create test-env pod. SECRETS_DESTINATION is with invalid value 'incorrect_secrets'"
export SECRETS_DESTINATION_KEY_VALUE=$KEY_VALUE_NOT_EXIST
deploy_test_env

pod_name=$(cli_get_pods_test_env | awk '{print $1}')

echo "Expecting secrets provider to fail with error 'CSPFK004E Environment variable 'SECRETS_DESTINATION' must be provided'"
$cli "logs $pod_name -c cyberark-secrets-provider | grep CSPFK004E"
