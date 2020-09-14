#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env without K8S_SECRETS environment variable"
export K8S_SECRETS_KEY_VALUE=$KEY_VALUE_NOT_EXIST
deploy_init_env

echo "Expecting for 'CrashLoopBackOff' state of pod test-env"
wait_for_it 600 "cli_get_pods_test_env | grep CrashLoopBackOff"

echo "Expecting secrets provider to fail with error 'CSPFK004E Environment variable K8S_SECRETS must be provided'"
$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^1$'"
pod_name=$($cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers" | awk '{print $1}')

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK004E"
