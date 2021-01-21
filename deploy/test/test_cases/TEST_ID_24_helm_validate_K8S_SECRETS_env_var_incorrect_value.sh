#!/bin/bash
set -euxo pipefail

# This test verifies that the Secrets Provider (deployed with Helm) mechanism still fails in case a K8S Secret does not exist and the proper errors appear in logs
setup_helm_environment

pushd ../../
  # Install HELM with a K8s Secret that does not exist
  export K8S_SECRETS="K8S_SECRET-non-existent-secret"
  export LABELS="app: test-helm"
  export DEBUG="true"

  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

echo "Expecting Secrets Provider to fail with debug message 'CSPFK004D Failed to retrieve k8s secret. Reason: secrets K8S_SECRET-non-existent-secret not found'"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-helm')"

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK004D"

echo "Expecting Secrets Provider to fail with error 'CSPFK020E Failed to retrieve k8s secret'"
$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK020E"
