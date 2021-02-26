#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

set_namespace "$APP_NAMESPACE_NAME"
deploy_init_env

# Source of truth is in deploy/policy/
# This file is run from deploy/test/test_cases/
secret_value="$(cat ../../policy/binary_data.pem)"

echo "Verifying pod test_env has environment variable 'BINARY_SECRET' with value '${secret_value}'"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"
verify_secret_value_in_pod "$pod_name" "BINARY_SECRET" "${secret_value}"
