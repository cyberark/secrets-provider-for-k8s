#!/bin/bash
set -euxo pipefail

# Restore secret to original value
set_namespace $CONJUR_NAMESPACE_NAME

configure_cli_pod
$cli exec $(get_conjur_cli_pod_name) -- conjur variable values add secrets/test_secret "supersecret"

set_namespace $TEST_APP_NAMESPACE_NAME

$cli delete secret dockerpullsecret --ignore-not-found=true

$cli delete clusterrole secrets-access-${UNIQUE_TEST_ID} --ignore-not-found=true

$cli delete secret test-k8s-secret --ignore-not-found=true

$cli delete serviceaccount ${TEST_APP_NAMESPACE_NAME}-sa --ignore-not-found=true

$cli delete rolebinding secrets-access-role-binding --ignore-not-found=true

if [ "${PLATFORM}" = "kubernetes" ]; then
  $cli delete deployment test-env --ignore-not-found=true
elif [ "${PLATFORM}" = "openshift" ]; then
  $cli delete deploymentconfig test-env --ignore-not-found=true
fi

$cli delete configmap conjur-master-ca-env --ignore-not-found=true

 echo "Verifying there are no (terminating) pods of type test-env"
 wait_for_it 600 "$cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^0$'"
