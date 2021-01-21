#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env without CONTAINER_MODE envrionment variable"
export CONTAINER_MODE_KEY_VALUE=$KEY_VALUE_NOT_EXIST
deploy_init_env

echo "Expecting secrets provider to fail with error 'CSPFK007E Setting SECRETS_DESTINATION environment variable to 'k8s_secrets' must run as init container'"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK007E"
