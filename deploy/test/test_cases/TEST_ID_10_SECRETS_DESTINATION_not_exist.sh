#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Create test-env pod. SECRETS_DESTINATION is with invalid value 'incorrect_secrets'"
export SECRETS_DESTINATION_KEY_VALUE=$KEY_VALUE_NOT_EXIST
deploy_init_env

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

echo "Expecting secrets provider to fail with error 'CSPFK004E Environment variable 'SECRETS_DESTINATION' must be provided'"
$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK004E"
