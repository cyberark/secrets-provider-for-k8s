#!/bin/bash
set -euxo pipefail

. utils.sh

announce "Generating Conjur policy."

pushd policy
  mkdir -p ./generated

  # NOTE: generated files are prefixed with the APP_NAMESPACE to allow for parallel CI
  ./templates/cluster-authn-svc-def.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.cluster-authn-svc.yml

  ./templates/conjur-authn.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.project-authn.yml

  ./templates/conjur-secrets.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.conjur-secrets.yml

  ./templates/app-identity-def.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.app-identity.yml

  ./templates/authn-any-policy-branch.template.sh.yml > generated/$APP_NAMESPACE_NAME.authn-any-policy-branch.yml
popd

# Create the random database password
password=$(openssl rand -hex 12)

if [[ "${DEPLOY_MASTER_CLUSTER}" == "true" ]]; then

  announce "Loading Conjur policy."

  set_namespace "$CONJUR_NAMESPACE_NAME"
  conjur_cli_pod=$(get_conjur_cli_pod_name)
  $cli_with_timeout "exec $conjur_cli_pod -- rm -rf /tmp/policy"
  $cli_with_timeout "cp ./policy $conjur_cli_pod:/tmp/policy"

  announce "Extracting openid configuration"
  JWKS_URI=$($cli_without_timeout get --raw /.well-known/openid-configuration | jq '.jwks_uri')
  ISSUER=$($cli_without_timeout get --raw /.well-known/openid-configuration | jq '.issuer')
  CA_CERT_B64=$($cli_without_timeout get configmap -n kube-system extension-apiserver-authentication -o jsonpath='{.data.client-ca-file}' | base64 -w0)
  announce "JWKS URI of this cluster is $JWKS_URI and Issuer is $ISSUER"

  announce "Allowing access to jwks uri for unauthenticated users"
  $cli_without_timeout delete clusterrolebinding oidc-reviewer --ignore-not-found
  $cli_without_timeout create clusterrolebinding oidc-reviewer --clusterrole=system:service-account-issuer-discovery --group=system:unauthenticated
  
  PROVIDER_URI="https://sts.windows.net/df242c82-fe4a-47e0-b0f4-e3cb7f8104f1/"

  $cli_with_timeout "exec $conjur_cli_pod -- \
    sh -c \"
      CONJUR_ADMIN_PASSWORD=${CONJUR_ADMIN_PASSWORD} \
      APP_NAMESPACE_NAME=${APP_NAMESPACE_NAME} \
      AUTHENTICATOR_ID='${AUTHENTICATOR_ID}' \
      JWKS_URI='${JWKS_URI}'\
      ISSUER='${ISSUER}'\
      CA_CERT_B64='${CA_CERT_B64}' \
      PROVIDER_URI='${PROVIDER_URI}' \
      /tmp/policy/load_policies.sh
    \""

  $cli_with_timeout "exec $conjur_cli_pod -- rm -rf ./tmp/policy"

  echo "Conjur policy loaded."
fi

announce "Ended load conjur policies"
