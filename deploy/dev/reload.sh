#!/bin/bash
set -xeuo pipefail

. utils.sh

main() {
  export DEV_HELM=${DEV_HELM:-"false"}

   # Clean-up previous run
  if [ "$(helm ls -aq | wc -l | tr -d ' ')" != 0 ]; then
    helm delete $(helm ls -aq)
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
    selector="role=follower"
    cert_location="/opt/conjur/etc/ssl/conjur.pem"
    if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
      selector="app=conjur-cli"
      cert_location="/home/cli/conjur-server.pem"
    fi

    conjur_pod_name="$(get_pod_name "$CONJUR_NAMESPACE_NAME" "$selector")"
    ssl_cert=$($cli_with_timeout "exec ${conjur_pod_name} --namespace $CONJUR_NAMESPACE_NAME -- cat $cert_location")

    export CONJUR_SSL_CERTIFICATE=$ssl_cert

    set_config_directory_path

    $cli_with_timeout "delete deployment init-env --ignore-not-found=true"

    deploy_env
  fi
}

main
