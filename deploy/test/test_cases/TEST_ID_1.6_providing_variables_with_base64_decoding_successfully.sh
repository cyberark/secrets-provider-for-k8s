#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

secret_value="secret-value"
environment_variable_name="VARIABLE_WITH_BASE64_SECRET"

set_namespace "$APP_NAMESPACE_NAME"
deploy_env

echo "Verifying pod test_env has environment variable '$environment_variable_name' with value '$secret_value'"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"
verify_secret_value_in_pod "$pod_name" "$environment_variable_name" "$secret_value"
