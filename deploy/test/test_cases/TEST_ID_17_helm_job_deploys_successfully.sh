#!/bin/bash
set -euxo pipefail

# This test verifies that the Secrets Provider Job deploys successfully and Conjur secret appears in pod
setup_helm_environment

pushd ../../
  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

# Deploy app to test against
deploy_helm_app

# Wait for Job completion
helm_chart_name="secrets-provider"
wait_for_job $helm_chart_name

app_pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"
verify_secret_value_in_pod $app_pod_name "TEST_SECRET" "supersecret"

secrets_provider_pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-helm')"
echo "Expecting the Secrets provider to succeed with proper success log 'CSPFK009I DAP/Conjur Secrets updated in Kubernetes successfully'"
$cli_with_timeout "logs $secrets_provider_pod_name | grep CSPFK009I"
