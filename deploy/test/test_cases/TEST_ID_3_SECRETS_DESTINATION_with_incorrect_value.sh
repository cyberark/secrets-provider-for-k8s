#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env with incorrect value for SECRETS_DESTINATION environment variable"
export SECRETS_DESTINATION_KEY_VALUE="SECRETS_DESTINATION SECRETS_DESTINATION_incorrect_value"
deploy_init_env

echo "Expecting secrets provider to fail with error 'CSPFK005E Provided incorrect value for environment variable SECRETS_DESTINATION'"
pod_name="$(get_pod_name ${APP_NAMESPACE_NAME} 'test-env')"

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK005E"
