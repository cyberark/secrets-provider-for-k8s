#!/bin/bash
set -euxo pipefail

# This test verifies that when the user declares that they will provide their own Service Account but that Service Account
# does not exist in the namespace the Secrets Provider will fail
setup_helm_environment

pushd ../../
  export CREATE_SERVICE_ACCOUNT="false"

  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

# Job should fail and not be completed
helm_chart_name="secrets-provider"
$cli_with_timeout "describe job $helm_chart_name | grep 'error looking up service account'"
