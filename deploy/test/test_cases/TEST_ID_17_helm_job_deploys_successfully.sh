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

# Check for Job completion
helm_chart_name="secrets-provider"
$cli_with_timeout wait --for=condition=complete job/$helm_chart_name

pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}' )
verify_secret_value_in_pod $pod_name "TEST_SECRET" "supersecret"

pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers | awk '{print $1}' )
echo "Expecting the Secrets provider to succeed with proper success log 'CSPFK009I DAP/Conjur Secrets updated in Kubernetes successfully'"
$cli_with_timeout "logs $pod_name | grep CSPFK009I"
