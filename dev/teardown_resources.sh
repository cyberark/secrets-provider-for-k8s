#!/bin/bash
set -euxo pipefail

. utils.sh

# Restore secret to original value
set_namespace $CONJUR_NAMESPACE_NAME

configure_cli_pod

set_namespace $APP_NAMESPACE_NAME

$cli_with_timeout "delete secret dockerpullsecret --ignore-not-found=true"

$cli_with_timeout "delete clusterrole secrets-access-${UNIQUE_TEST_ID} --ignore-not-found=true"

$cli_with_timeout "delete secret test-k8s-secret --ignore-not-found=true"
$cli_with_timeout "delete secret sigal-secret --ignore-not-found=true"

$cli_with_timeout "delete serviceaccount ${APP_NAMESPACE_NAME}-sa --ignore-not-found=true"

$cli_with_timeout "delete rolebinding secrets-access-role-binding --ignore-not-found=true"

if [ "${PLATFORM}" = "kubernetes" ]; then
  $cli_with_timeout "delete deployment test-env --ignore-not-found=true"
elif [ "${PLATFORM}" = "openshift" ]; then
  $cli_with_timeout "delete deploymentconfig test-env --ignore-not-found=true"
fi

$cli_with_timeout "delete configmap conjur-master-ca-env --ignore-not-found=true"

