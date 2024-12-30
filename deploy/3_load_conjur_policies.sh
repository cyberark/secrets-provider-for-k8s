#!/bin/bash
set -euxo pipefail

. utils.sh

announce "Generating Conjur policy."

pushd policy
  mkdir -p ./generated

  # NOTE: generated files are prefixed with the APP_NAMESPACE to allow for parallel CI
  ./templates/cluster-authn-svc-def.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.cluster-authn-svc.yml

  ./templates/conjur-authn-k8s.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.project-authn.yml

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
  
  $cli_with_timeout "exec $conjur_cli_pod -- \
    sh -c \"
      CONJUR_ADMIN_PASSWORD=${CONJUR_ADMIN_PASSWORD} \
      APP_NAMESPACE_NAME=${APP_NAMESPACE_NAME} \
      /tmp/policy/load_policies.sh
    \""

  $cli_with_timeout "exec $conjur_cli_pod -- rm -rf ./tmp/policy"

  echo "Conjur policy loaded."
fi

announce "Ended load conjur policies"
