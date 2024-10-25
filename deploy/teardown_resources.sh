#!/bin/bash
set -euxo pipefail

. "$(dirname "${0}")/utils.sh"

announce "Teardown resources"
# Restore secret to original value
set_namespace $CONJUR_NAMESPACE_NAME

configure_cli_pod

helm_ci_path="../helm/secrets-provider/ci"
if [[ "${DEV}" = "false" || "${RUN_IN_DOCKER}" = "true" ]]; then
  helm_ci_path="../../../helm/secrets-provider/ci"
fi
pushd $helm_ci_path
  find . -type f -name "*${UNIQUE_TEST_ID}.yaml" -delete
popd

# Delete Helm Chart if already exists
set_namespace $APP_NAMESPACE_NAME
if [ "$(helm ls -aq | wc -l | tr -d ' ')" != 0 ]; then
  helm delete $(helm ls -aq)
fi

set_namespace $CONJUR_NAMESPACE_NAME

$cli_with_timeout "exec $(get_conjur_cli_pod_name) -- conjur variable set -i secrets/test_secret -v \"supersecret\""

set_namespace $APP_NAMESPACE_NAME

$cli_with_timeout "delete secret dockerpullsecret --ignore-not-found=true"

$cli_with_timeout "delete clusterrole secrets-access-${UNIQUE_TEST_ID} --ignore-not-found=true"

$cli_with_timeout "delete role another-secrets-provider-role --ignore-not-found=true"

$cli_with_timeout "delete secret test-k8s-secret --ignore-not-found=true"

$cli_with_timeout "delete secret test-k8s-secret-fetch-all --ignore-not-found=true"

$cli_with_timeout "delete secret test-k8s-secret-fetch-all-base64 --ignore-not-found=true"

$cli_with_timeout "delete secret another-test-k8s-secret --ignore-not-found=true"

$cli_with_timeout "delete serviceaccount ${APP_NAMESPACE_NAME}-sa --ignore-not-found=true"

$cli_with_timeout "delete serviceaccount another-secrets-provider-service-account --ignore-not-found=true"

$cli_with_timeout "delete rolebinding secrets-access-role-binding --ignore-not-found=true"

$cli_with_timeout "delete rolebinding another-secrets-provider-role-binding --ignore-not-found=true"

$cli_with_timeout "delete deployment test-env --ignore-not-found=true"
$cli_with_timeout "delete deployment another-test-env --ignore-not-found=true"

$cli_with_timeout "delete configmap conjur-master-ca-env --ignore-not-found=true"

echo "Verifying there are no (terminating) pods of type test-env"
$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^0$'"

$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=another-test-env --no-headers | wc -l | tr -d ' ' | grep '^0$'"

echo "Verifying there are no (terminating) pods for Secrets Provider deployed with Helm"
$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers | wc -l | tr -d ' ' | grep '^0$'"
