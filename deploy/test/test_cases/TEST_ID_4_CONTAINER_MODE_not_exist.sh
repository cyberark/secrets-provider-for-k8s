#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env without CONTAINER_MODE envrionment variable"
export CONTAINER_MODE_KEY_VALUE=$KEY_VALUE_NOT_EXIST
deploy_init_env

echo "Expecting secrets provider to succeed as an init container"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep \"CSPFK014I Authenticator setting CONTAINER_MODE provided by default\""
verify_secret_value_in_pod $pod_name TEST_SECRET supersecret
