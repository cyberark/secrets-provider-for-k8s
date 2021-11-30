#!/bin/bash
set -euxo pipefail

# This test verifies that if the user does not override Helm defaults, Helm will name K8s resources by their defaults and
# the Secrets Provider will deploy and complete successfully
setup_helm_environment

pushd ../../
  export DEBUG="false"
  export LABELS="app: test-helm"
  export K8S_SECRETS="test-k8s-secret"
  export CONJUR_ACCOUNT="cucumber"
  export CONJUR_AUTHN_URL="https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local/api/authn-k8s/${AUTHENTICATOR_ID}"
  export CONJUR_AUTHN_LOGIN="host/conjur/authn-k8s/${AUTHENTICATOR_ID}/apps/${APP_NAMESPACE_NAME}/*/*"

  fill_helm_chart_no_override_defaults
  helm install -f "../helm/secrets-provider/ci/take-default-test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

# Validate that known defaults were taken if not supplied
$cli_with_timeout "get ServiceAccount secrets-provider-service-account"
$cli_with_timeout "get Role secrets-provider-role"
$cli_with_timeout "get RoleBinding secrets-provider-role-binding"
$cli_with_timeout "get ConfigMap cert-config-map"

# Validate that the Secrets Provider took the default image configurations if not supplied and was deployed successfully
$cli_with_timeout "describe job secrets-provider | grep 'cyberark/secrets-provider-for-k8s:1.2.0'" | awk '{print $2}' && $cli_with_timeout "get job secrets-provider -o jsonpath={.status.succeeded}"
