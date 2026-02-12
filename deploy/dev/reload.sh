#!/bin/bash
set -xeuo pipefail

. utils.sh

source "../bootstrap.env"

main() {
  export DEV_HELM=${DEV_HELM:-"false"}

   # Clean-up previous run
  if [ "$(helm ls -q | wc -l | tr -d ' ')" != 0 ]; then
    helm delete $(helm ls -q)
  fi
  $cli_with_timeout "delete deployment test-env --ignore-not-found=true"

  pushd ..
    ./bin/build
  popd

  set_namespace $APP_NAMESPACE_NAME

  if [ "${DEV_HELM}" = "true" ]; then
    setup_helm_environment

    export IMAGE_PULL_POLICY="Never"
    export IMAGE="secrets-provider-for-k8s"
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
