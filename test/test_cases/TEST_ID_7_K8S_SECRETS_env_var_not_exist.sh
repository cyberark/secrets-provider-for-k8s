#!/bin/bash
set -euxo pipefail

source $TEST_CASES_UTILS

create_secret_access_role

create_secret_access_role_binding

echo "Deploying test_env without K8S_SECRETS environment variable"
export K8S_SECRETS_KEY_VALUE=" "
deploy_test_env

echo "Expecting for 'CrashLoopBackOff' state of pod test-env"
wait_for_it 600 "$cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | grep CrashLoopBackOff"

echo "Expecting secrets provider to fail with error 'CSPFK004E Environment variable K8S_SECRETS must be provided'"
pod_name=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')
wait_for_it 600 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK004E'"

