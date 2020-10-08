#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

export CONJUR_AUTHN_LOGIN="host/some-policy/non-existing-namespace/*/*"

deploy_init_env

echo "Expecting secrets provider to fail with error CAKC015 Login failed"
$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^1$'"
pod_name=$($cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers" | awk '{print $1}')

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CAKC015"
