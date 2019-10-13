#!/bin/bash
set -euxo pipefail

source $TEST_CASES_UTILS

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env with incorrect value for K8S_SECRETS envrionment variable"
export K8S_SECRETS_KEY_VALUE="${K8S_SECRETS_KEY_VALUE:-"          - name: K8S_SECRETS"$'\n'"            value: K8S_SECRETS_invalid_value"}"
deploy_test_env

echo "Expecting secrets provider to fail with debug message 'CSPFK004D Failed to retrieve k8s secret. Reason: secrets K8S_SECRETS_invalid_value not found'"
pod_name=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')
wait_for_it 600 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK004D'"

echo "Expecting secrets provider to fail with error 'CSPFK020E Failed to retrieve k8s secret'"
$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK020E'
