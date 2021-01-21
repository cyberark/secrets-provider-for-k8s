#!/bin/bash
set -euxo pipefail

# This test verifies that providing secrets from Conjur doesn't remove other keys
# from the k8s secret (e.g the key 'non-conjur-key' in k8s-secret.yml)

create_secret_access_role

create_secret_access_role_binding

deploy_init_env

k8s_secret_key="NON_CONJUR_SECRET"
secret_value="some-value"

echo "Verifying pod test_env has environment variable '$k8s_secret_key' with value '$secret_value'"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

verify_secret_value_in_pod "$pod_name" $k8s_secret_key "$secret_value"
