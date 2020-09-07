#!/bin/bash
set -euxo pipefail

# This test verifies that the Secrets Provider Job deploys successfully and Conjur secret appears in pod
setup_helm_environment

pushd ../../
  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur.pem"
popd

# Deploy app to test against
deploy_helm_app

# Wait for Job completion
helm_chart_name="secrets-provider"
$cli_with_timeout "get job/$helm_chart_name -o=jsonpath='{.status.conditions[*].type}' | grep Complete"

$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^1$'"
app_pod_name=$($cli_without_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}' )
verify_secret_value_in_pod $app_pod_name "TEST_SECRET" "supersecret"

$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers | wc -l | tr -d ' ' | grep '^1$'"
secrets_provider_pod_name=$($cli_without_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers | awk '{print $1}' )
echo "Expecting the Secrets provider to succeed with proper success log 'CSPFK009I DAP/Conjur Secrets updated in Kubernetes successfully'"
$cli_with_timeout "logs $secrets_provider_pod_name | grep CSPFK009I"
