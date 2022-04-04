#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

deploy_k8s_rotation_env

pod_name1="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'supersecret'"
verify_secret_value_in_pod $pod_name1 TEST_SECRET supersecret

set_conjur_secret secrets/test_secret secret2
sleep 10

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'secret2'"
verify_secret_value_in_pod $pod_name1 TEST_SECRET secret2
