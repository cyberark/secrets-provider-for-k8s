#!/bin/bash
set -xeuo pipefail

. utils.sh

# Clean up when script completes and fails
finish() {
  # There is a TRAP in test_in_docker.sh to account for Docker deployments so we do not need to add another one here
  # Stop the running processes
  if [[ $RUN_IN_DOCKER = false && $DEV = false ]]; then
    announce 'Wrapping up and removing environment'
    repo_root_path=$(git rev-parse --show-toplevel)
    "$repo_root_path/deploy/stop"
    pushd $repo_root_path/kubernetes-conjur-deploy-$UNIQUE_TEST_ID
      ./stop
    popd
    # Remove the deploy directory
    rm -rf "$repo_root_path/kubernetes-conjur-deploy-$UNIQUE_TEST_ID"
  fi
}
trap finish EXIT

# Will print platform regardless of GKE or Openshift
# Will only print version if Openshift
announce "Running tests on: ${PLATFORM} ${OPENSHIFT_VERSION:-}"

if [ "${DEV}" = "false" ]; then
  ./platform_login.sh
fi

./1_check_dependencies.sh

if [ "${DEV}" = "false" ]; then
  ./stop
fi

./2_create_app_namespace.sh

if [[ "${DEPLOY_MASTER_CLUSTER}" = "true" ]]; then
  ./3_load_conjur_policies.sh
  if [[ "$CONJUR_DEPLOYMENT" != "cloud" ]]; then
    ./4_init_conjur_cert_authority.sh
  fi
fi

set_namespace $APP_NAMESPACE_NAME

echo "Publish docker image"

# No implicit ./bin/build — CI loads from another stage via SECRETS_PROVIDER_SOURCE_IMAGE; else prefer :latest then :dev.
SP_SRC="${SECRETS_PROVIDER_SOURCE_IMAGE:-}"
if [[ -z "${SP_SRC}" ]]; then
  if docker image inspect secrets-provider-for-k8s:latest >/dev/null 2>&1; then
    SP_SRC=secrets-provider-for-k8s:latest
  elif docker image inspect secrets-provider-for-k8s:dev >/dev/null 2>&1; then
    SP_SRC=secrets-provider-for-k8s:dev
  fi
fi
if [[ -z "${SP_SRC}" ]] || ! docker image inspect "${SP_SRC}" >/dev/null 2>&1; then
  echo "secrets-provider image not found (tried SECRETS_PROVIDER_SOURCE_IMAGE, secrets-provider-for-k8s:latest, secrets-provider-for-k8s:dev)." >&2
  echo "Set SECRETS_PROVIDER_SOURCE_IMAGE for CI, or run ./bin/build locally." >&2
  exit 1
fi

# Tag and push/load image based on environment
if [[ "${DEV}" = "false" && "${KIND}" != "true" ]]; then
  # Push to registry for CI environments
  docker tag "${SP_SRC}" \
    "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"
  docker push "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"
  
  # Also push debian to avoid Dockerhub pull restrictions
  docker pull debian:latest
  docker tag debian:latest "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/debian:latest"
  docker push "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/debian:latest"
  
  # Set CONJUR_APP_IMAGE for deployment manifests
  export CONJUR_APP_IMAGE="${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"
else
  # Local dev or Kind - tag locally (explicit :latest matches dev manifests / kubelet lookup)
  docker tag "${SP_SRC}" \
    "${APP_NAMESPACE_NAME}/secrets-provider:latest"

  # Set CONJUR_APP_IMAGE for deployment manifests (no registry prefix for Kind/dev)
  export CONJUR_APP_IMAGE="${APP_NAMESPACE_NAME}/secrets-provider"
fi

# For Kind clusters, load the image directly
if [[ "${KIND}" = "true" ]]; then
  export PATH="${PATH}:/root/go/bin"
  source "$(dirname "${BASH_SOURCE[0]}")/kind_helpers.sh"
  kind_cluster="${KIND_CLUSTER_NAME:-kind}"
  docker pull debian:latest
  kind_load_image "${kind_cluster}" "${APP_NAMESPACE_NAME}/secrets-provider:latest"
  kind_load_image "${kind_cluster}" "debian:latest"
fi

fetch_ssl_from_conjur

if [[ "${DEV}" = "false" ]]; then
  pushd ./test/test_cases > /dev/null
    ./run_tests.sh
  popd > /dev/null
else
  ./dev/5_load_environment.sh
fi
