#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env with incorrect value for K8S_SECRETS envrionment variable"
export K8S_SECRETS_KEY_VALUE="K8S_SECRETS K8S_SECRETS_invalid_value"
deploy_test_env

echo "Expecting secrets provider to fail with debug message 'CSPFK004D Failed to retrieve k8s secret. Reason: secrets K8S_SECRETS_invalid_value not found'"
pod_name=$(cli_get_pods_test_env | awk '{print $1}')
wait_for_it 600 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK004D'"

echo "Expecting secrets provider to fail with error 'CSPFK020E Failed to retrieve k8s secret'"
$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK020E'
