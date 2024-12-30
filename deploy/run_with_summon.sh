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
  ./4_init_conjur_cert_authority.sh
fi

set_namespace $APP_NAMESPACE_NAME

echo "Publish docker image"

if [ "${DEV}" = "false"  ]; then
  docker tag "secrets-provider-for-k8s:dev" \
    "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"

  docker push "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"

  # Make sure debian is pushed to the internal registry to avoid Dockerhub pull restrictions
  docker pull debian:latest
  docker tag debian:latest "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/debian:latest"
  docker push "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/debian:latest"
else
  docker tag "secrets-provider-for-k8s:dev" \
    "${APP_NAMESPACE_NAME}/secrets-provider"
fi

selector="role=follower"
cert_location="/opt/conjur/etc/ssl/conjur.pem"
if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
    selector="app=conjur-cli"
    cert_location="/home/cli/conjur-server.pem"
fi
conjur_pod_name="$(get_pod_name "$CONJUR_NAMESPACE_NAME" "$selector")"
ssl_cert=$($cli_with_timeout "exec ${conjur_pod_name} --namespace $CONJUR_NAMESPACE_NAME -- cat $cert_location")

export CONJUR_SSL_CERTIFICATE=$ssl_cert

if [[ "${DEV}" = "false" ]]; then
  pushd ./test/test_cases > /dev/null
    ./run_tests.sh
  popd > /dev/null
else
  ./dev/5_load_environment.sh
fi
