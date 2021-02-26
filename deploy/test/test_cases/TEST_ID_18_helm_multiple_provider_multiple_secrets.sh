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
  helm install -f "../helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

helm_chart_name="secrets-provider"
wait_for_job $helm_chart_name

deploy_helm_app

# Deployed twice to ensure conjur.pem exists
setup_helm_environment
# Deploy second Secrets Provider Job
pushd ../../
  export SECRETS_PROVIDER_ROLE=another-secrets-provider-role
  export SECRETS_PROVIDER_ROLE_BINDING=another-secrets-provider-role-binding
  export SERVICE_ACCOUNT=another-secrets-provider-service-account
  export K8S_SECRETS=another-test-k8s-secret
  export SECRETS_PROVIDER_SSL_CONFIG_MAP=another-secrets-provider-ssl-config-map
  fill_helm_chart "another-"
  helm install -f "../helm/secrets-provider/ci/another-test-values-$UNIQUE_TEST_ID.yaml" \
    another-secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

# Wait for Job completion
helm_chart_name="another-secrets-provider"
wait_for_job $helm_chart_name

export K8S_SECRET=another-test-k8s-secret
deploy_helm_app "another-"

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"
verify_secret_value_in_pod $pod_name "TEST_SECRET" "supersecret"

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=another-test-env')"
verify_secret_value_in_pod $pod_name "another-TEST_SECRET" "another-some-secret-value"
