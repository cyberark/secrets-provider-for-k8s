#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

export CONJUR_AUTHN_LOGIN="host/some-apps/${APP_NAMESPACE_NAME}/*/*"

deploy_init_env

echo "Verifying pod test_env has environment variable 'TEST_SECRET' with value 'supersecret'"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

verify_secret_value_in_pod $pod_name TEST_SECRET supersecret
