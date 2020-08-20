#!/bin/bash
set -euxo pipefail

. utils.sh

main() {
  export DEV_HELM=${DEV_HELM:-"false"}
  ./teardown_resources.sh

  set_namespace "$APP_NAMESPACE_NAME"

  if [ "${DEV_HELM}" = "true" ]; then
    setup_helm_environment

    create_k8s_secret
    export IMAGE_PULL_POLICY="Never"
    export IMAGE="secrets-provider-for-k8s"
    export TAG="latest"
    deploy_chart

    deploy_helm_app
  else
    create_k8s_secret

    create_secret_access_role

    create_secret_access_role_binding

    deploy_init_env
  fi
}

create_k8s_secret() {
  announce "Creating K8s Secret."

  set_config_directory_path

  echo "Create secret k8s-secret"
  $cli_with_timeout create -f $CONFIG_DIR/k8s-secret.yml
}

main
