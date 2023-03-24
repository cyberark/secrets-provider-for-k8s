#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

export SECRETS_MODE="k8s-rotation"
deploy_env

pod_name1="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'supersecret'"
verify_secret_value_in_pod $pod_name1 TEST_SECRET supersecret

echo "Verify pod $pod_name1 has environment variable 'VARIABLE_WITH_BASE64_SECRET' with value 'secret-value'"
verify_secret_value_in_pod $pod_name1 VARIABLE_WITH_BASE64_SECRET secret-value

set_conjur_secret secrets/test_secret secret2
set_conjur_secret secrets/encoded "$(echo "secret-value2" | base64)"
sleep 10

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'secret2'"
verify_secret_value_in_pod $pod_name1 TEST_SECRET secret2
echo "Verify pod $pod_name1 has environment variable 'VARIABLE_WITH_BASE64_SECRET' with value 'secret-value2'"
verify_secret_value_in_pod $pod_name1 VARIABLE_WITH_BASE64_SECRET secret-value2

# Note: We're not testing secrets deletion here like we do in TEST_ID_28_push_to_file_secrets_rotation. This is because removing the
# secret values from K8s will cause the pod to fail on startup due to the missing secretKeyRefs. We would need another way to test this
# other than checking that the environment variable is cleared. Being that rotation is mainly used with Push-to-File, it may not be
# worth going through the effort to develop tests for this scenario.
