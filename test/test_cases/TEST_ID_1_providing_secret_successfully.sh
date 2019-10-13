#!/bin/bash
set -euxo pipefail

source $TEST_CASES_UTILS

create_secret_access_role

create_secret_access_role_binding

deploy_test_env

echo "Verifying pod test_env has environment variable 'TEST_SECRET' with value 'supersecret'"
pod_name=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')
wait_for_it 30 "  $cli exec -n $TEST_APP_NAMESPACE_NAME ${pod_name} printenv | grep TEST_SECRET | cut -d \"=\" -f 2 | grep 'supersecret'"

