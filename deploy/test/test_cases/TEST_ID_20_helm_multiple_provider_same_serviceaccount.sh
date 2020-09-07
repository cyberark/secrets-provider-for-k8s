#!/bin/bash
set -euxo pipefail

# This test verifies that two Secrets Provider Jobs can run with same Service Account successfully in the same namespace
setup_helm_environment

pushd ../../
  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur.pem"
popd

# Check for Job completion
helm_chart_name="secrets-provider"
$cli_with_timeout "get job/$helm_chart_name -o=jsonpath='{.status.conditions[*].type}' | grep Complete"

setup_helm_environment

pushd ../../
  export CREATE_SERVICE_ACCOUNT="false"
  export LABELS="app: another-test-helm"
  # Supply same Service Account resource that was created above
  export SERVICE_ACCOUNT=secrets-provider-service-account
  export SECRETS_PROVIDER_SSL_CONFIG_MAP=another-secrets-provider-ssl-config-map
  fill_helm_chart "another-"
  helm install -f "../helm/secrets-provider/ci/another-test-values.yaml" \
    another-secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur.pem"
popd

helm_chart_name="another-secrets-provider"
$cli_with_timeout "get job/$helm_chart_name -o=jsonpath='{.status.conditions[*].type}' | grep Complete"

# Verify that another-secrets-provider runs with the correct Service Account
$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=another-test-helm --no-headers | grep another-secrets-provider"
pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers | awk '{print $1}' | head -1)
$cli_with_timeout get pods/$pod_name -o yaml | grep "serviceAccount: secrets-provider-service-account"

# Deploy app to test against
deploy_helm_app

$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | grep Running"
pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}' | head -1)
verify_secret_value_in_pod $pod_name "TEST_SECRET" "supersecret"
