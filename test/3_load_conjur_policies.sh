#!/bin/bash
set -euo pipefail

. utils.sh

announce "Generating Conjur policy."

pushd policy
  mkdir -p ./generated

  # NOTE: generated files are prefixed with the test app namespace to allow for parallel CI

  ./templates/cluster-authn-svc-def.template.sh.yml > ./generated/$TEST_APP_NAMESPACE_NAME.cluster-authn-svc.yml

  ./templates/project-authn-def.template.sh.yml > ./generated/$TEST_APP_NAMESPACE_NAME.project-authn.yml

  ./templates/conjur-secrets.template.sh.yml > ./generated/$TEST_APP_NAMESPACE_NAME.conjur-secrets.yml

  ./templates/app-identity-def.template.sh.yml > ./generated/$TEST_APP_NAMESPACE_NAME.app-identity.yml

  ./templates/conjur-pet-store-secrets.template.sh.yml > generated/$TEST_APP_NAMESPACE_NAME.conjur-pet-store-secrets.yml

  ./templates/authn-any-policy-branch.template.sh.yml > generated/$TEST_APP_NAMESPACE_NAME.authn-any-policy-branch.yml

popd

# Create the random database password
password=$(openssl rand -hex 12)

if [[ "${DEPLOY_MASTER_CLUSTER}" == "true" ]]; then

  announce "Loading Conjur policy."

  set_namespace "$CONJUR_NAMESPACE_NAME"
  conjur_cli_pod=$(get_conjur_cli_pod_name)
  $cli "exec $conjur_cli_pod -- rm -rf /policy"
  $cli "cp ./policy $conjur_cli_pod:/policy"

  $cli "exec $conjur_cli_pod -- \
    bash -c \"
      CONJUR_ADMIN_PASSWORD=${CONJUR_ADMIN_PASSWORD} \
      TEST_APP_NAMESPACE_NAME=${TEST_APP_NAMESPACE_NAME} \
      CONJUR_VERSION=${CONJUR_VERSION} \
      POSTGRES_USERNAME=${POSTGRES_USERNAME} \
      POSTGRES_PASSWORD=${POSTGRES_PASSWORD} \
      /policy/load_policies.sh
    \""

  $cli "exec $conjur_cli_pod -- rm -rf ./policy"

  echo "Conjur policy loaded."
fi

announce "Ended load conjur policies"
