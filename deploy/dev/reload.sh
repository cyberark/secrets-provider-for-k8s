#!/bin/bash
set -euo pipefail

. utils.sh

_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${_REPO_ROOT}/bootstrap.env"

main() {
  export DEV_HELM=${DEV_HELM:-"false"}
  set_namespace "$APP_NAMESPACE_NAME"

   # Clean-up previous run
  if [ "$(helm ls -q | wc -l | tr -d ' ')" != 0 ]; then
    helm delete $(helm ls -q)
  fi
  $cli_with_timeout "delete deployment test-env --ignore-not-found=true --namespace=${APP_NAMESPACE_NAME}"

  pushd ..
    ./bin/build
  popd

  if [[ "${KIND}" = "true" ]]; then
    . "${_REPO_ROOT}/deploy/kind_helpers.sh"
    if [[ -z "${KIND_CLUSTER_NAME:-}" ]]; then
      KIND_CLUSTER_NAME="$(kind_sanitize_cluster_name "${UNIQUE_TEST_ID:-secrets-provider-test}")"
    fi
    # Tag and push the built secrets-provider-for-k8s image to the Kind cluster
    local secrets_provider_image="${APP_NAMESPACE_NAME}/secrets-provider:latest"
    export SECRETS_PROVIDER_IMAGE="${APP_NAMESPACE_NAME}/secrets-provider"
    docker tag "secrets-provider-for-k8s:latest" "${secrets_provider_image}"
    kind_load_image "${KIND_CLUSTER_NAME}" "${secrets_provider_image}"
  fi

  if [ "${DEV_HELM}" = "true" ]; then
    setup_helm_environment

    export IMAGE_PULL_POLICY="IfNotPresent"
    export IMAGE="${IMAGE:-${APP_NAMESPACE_NAME}/secrets-provider}"
    export TAG="latest"
    deploy_chart

    deploy_helm_app
  else
    fetch_ssl_from_conjur
    if [[ "$CONJUR_DEPLOYMENT" = "cloud" ]]; then
      cloud_login
    fi

    set_config_directory_path

    $cli_with_timeout "delete deployment init-env --ignore-not-found=true"

    deploy_env
  fi
}

main
