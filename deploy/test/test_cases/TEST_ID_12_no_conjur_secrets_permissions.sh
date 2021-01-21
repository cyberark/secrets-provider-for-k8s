#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

export CONJUR_AUTHN_LOGIN="host/conjur/authn-k8s/${AUTHENTICATOR_ID}/apps/${APP_NAMESPACE_NAME}/service_account/${APP_NAMESPACE_NAME}-sa"

deploy_init_env

echo "Expecting secrets provider to fail with error CSPFK034E Failed to retrieve Conjur secrets"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK034E"
