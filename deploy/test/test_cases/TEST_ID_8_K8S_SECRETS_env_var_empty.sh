#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env with empty value for K8S_SECRETS envrionment variable"
export K8S_SECRETS_KEY_VALUE="K8S_SECRETS"
deploy_init_env

echo "Expecting for CrashLoopBackOff state of pod test-env"
wait_for_it 600 "$(get_pods_info "$APP_NAMESPACE_NAME" test-env) | grep CrashLoopBackOff"

echo "Expecting Secrets provider to fail with error 'CSPFK004E Environment variable K8S_SECRETS must be provided'"
pod_name="$(get_pod_name "${APP_NAMESPACE_NAME}" 'test-env')"

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK004E"
