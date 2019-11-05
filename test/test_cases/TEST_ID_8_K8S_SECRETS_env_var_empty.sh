#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env with empty value for K8S_SECRETS envrionment variable"
export K8S_SECRETS_KEY_VALUE="K8S_SECRETS"
deploy_test_env

echo "Expecting for CrashLoopBackOff state of pod test-env"
wait_for_it 600 "cli_get_pods_test_env | grep CrashLoopBackOff"

echo "Expecting Secrets provider to fail with error 'CSPFK004E Environment variable K8S_SECRETS must be provided'"
pod_name=$(cli_get_pods_test_env | awk '{print $1}')
wait_for_it 600 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK004E'"
