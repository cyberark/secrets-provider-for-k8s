#!/bin/bash
set -euxo pipefail

# This test verifies that two Secrets Provider Jobs deploy successfully in the same namespace
setup_helm_environment

echo "Create second secret"
create_k8s_secret_for_helm_deployment
set_conjur_secret secrets/another_test_secret another-some-secret-value

# Deploy first Secrets Provider Job
pushd ../../
  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur.pem"
popd

helm_chart_name="secrets-provider"
$cli_with_timeout wait --for=condition=complete job/$helm_chart_name

deploy_helm_app

# Deploy second Secrets Provider Job
pushd ../../
  export SECRETS_PROVIDER_ROLE=another-secrets-provider-role
  export SECRETS_PROVIDER_ROLE_BINDING=another-secrets-provider-role-binding
  export SERVICE_ACCOUNT=another-secrets-provider-service-account
  export K8S_SECRETS=another-test-k8s-secret
  export SECRETS_PROVIDER_SSL_CONFIG_MAP=another-secrets-provider-ssl-config-map
  fill_helm_chart "another-"
  helm install -f "../helm/secrets-provider/ci/another-test-values.yaml" \
    another-secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur.pem"
popd

helm_chart_name="another-secrets-provider"
$cli_with_timeout wait --for=condition=complete job/$helm_chart_name

export K8S_SECRET=another-test-k8s-secret
deploy_helm_app "another-"

$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | grep Running"
pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}' | head -1)
verify_secret_value_in_pod $pod_name "TEST_SECRET" "supersecret"

$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=another-test-env --no-headers | grep Running"
pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=another-test-env --no-headers | awk '{print $1}' | head -1)
verify_secret_value_in_pod $pod_name "another-TEST_SECRET" "another-some-secret-value"
