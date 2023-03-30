#!/bin/bash
set -euo pipefail

create_secret_access_role

create_secret_access_role_binding

# Generate a large base64 encoded string (> 65k characters)
secret_value=$(openssl rand -base64 $((66 * 2**10)) | tr -d '\n')
encoded_secret_value=$(echo "$secret_value" | base64)
environment_variable_name="VARIABLE_WITH_BASE64_SECRET"

# Set the encoded secret value in Conjur
set_conjur_secret "secrets/encoded" "$encoded_secret_value"

set_namespace "$APP_NAMESPACE_NAME"
deploy_env

echo "Verifying pod test_env has environment variable '$environment_variable_name' with expected value"
test_pod="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"
actual_value=$($cli_with_timeout "exec $test_pod -- printenv | grep VARIABLE_WITH_BASE64_SECRET | cut -d= -f2")

if [[ "$actual_value" == "$secret_value" ]]; then
    echo "$environment_variable_name is set correctly"
    # Reset the secret value to the original value for subsequent tests
    set_conjur_secret secrets/encoded "$(echo "secret-value" | tr -d '\n' | base64)" # == "c2VjcmV0LXZhbHVl"
else
    echo "$environment_variable_name is not set correctly"
    exit 1
fi
