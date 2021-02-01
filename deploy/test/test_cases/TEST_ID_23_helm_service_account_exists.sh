#!/bin/bash
set -euxo pipefail

# This test verifies that when the user declares that they will provide their own Service Account and that Service Account exists
# the Secrets Provider will deploy and complete successfully
setup_helm_environment

create_k8s_role "another-"

pushd ../../
  # RBAC will not be created by the Secrets Provider Helm Chart
  export CREATE_SERVICE_ACCOUNT="false"

  # These roles should not be created because of the above configuration
  export SERVICE_ACCOUNT="another-secrets-provider-service-account"

  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

## Validate that resources were not created
$cli_with_timeout "get serviceaccount secrets-provider-service-account --no-headers 2>/dev/null || true | wc -l | tr -d ' ' | grep '^0$'"
$cli_with_timeout "get role secrets-provider-role --no-headers 2>/dev/null || true | wc -l | tr -d ' ' | grep '^0$'"
$cli_with_timeout "get rolebinding secrets-provider-role-binding --no-headers 2>/dev/null || true | wc -l | tr -d ' ' | grep '^0$'"

# Job will complete successfully because provided Service Account exists
helm_chart_name="secrets-provider"
wait_for_job $helm_chart_name
