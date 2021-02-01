#!/bin/bash
set -euxo pipefail

# This test verifies that two Secrets Provider Jobs can run with same Service Account successfully in the same namespace
setup_helm_environment

pushd ../../
  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

# Check for Job completion
helm_chart_name="secrets-provider"
wait_for_job $helm_chart_name

setup_helm_environment

pushd ../../
  export CREATE_SERVICE_ACCOUNT="false"
  export LABELS="app: another-test-helm"
  # Supply same Service Account resource that was created above
  export SERVICE_ACCOUNT=secrets-provider-service-account
  export SECRETS_PROVIDER_SSL_CONFIG_MAP=another-secrets-provider-ssl-config-map
  fill_helm_chart "another-"
  helm install -f "../helm/secrets-provider/ci/another-test-values-$UNIQUE_TEST_ID.yaml" \
    another-secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

helm_chart_name="another-secrets-provider"
wait_for_job $helm_chart_name

# Verify that another-secrets-provider runs with the correct Service Account
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-helm')"
$cli_with_timeout get pods/$pod_name -o yaml | grep "serviceAccount: secrets-provider-service-account"

# Deploy app to test against
deploy_helm_app

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"
verify_secret_value_in_pod $pod_name "TEST_SECRET" "supersecret"
