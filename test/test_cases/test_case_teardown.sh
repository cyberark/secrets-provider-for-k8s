#!/bin/bash
set -euxo pipefail

# Restore secret to original value
set_namespace $CONJUR_NAMESPACE_NAME

configure_cli_pod
$cli exec $(get_conjur_cli_pod_name) -- conjur variable values add secrets/test_secret "supersecret"

set_namespace $TEST_APP_NAMESPACE_NAME

$cli delete secret dockerpullsecret --ignore-not-found=true

$cli delete clusterrole secrets-access --ignore-not-found=true

$cli delete secret test-k8s-secret --ignore-not-found=true

$cli delete serviceaccount ${TEST_APP_NAMESPACE_NAME}-sa --ignore-not-found=true

$cli delete rolebinding secrets-access-role-binding --ignore-not-found=true

$cli delete deploymentconfig test-env --ignore-not-found=true

$cli delete configmap conjur-master-ca-env --ignore-not-found=true
