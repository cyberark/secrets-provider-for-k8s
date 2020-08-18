#!/bin/bash
set -euxo pipefail

. utils.sh

main() {
  export DEV_HELM=${DEV_HELM:-"false"}
  ./teardown_resources.sh

  set_namespace "$APP_NAMESPACE_NAME"

  configure_secret

  deploy_env
}

create_k8s_secret() {
  announce "Creating K8s Secret."

  export CONFIG_DIR="$PWD/config/k8s"
  if [[ "$PLATFORM" = "openshift" ]]; then
      export CONFIG_DIR="$PWD/config/openshift"
  fi

  echo "Create secret k8s-secret"
  $cli_with_timeout create -f $CONFIG_DIR/k8s-secret.yml

  create_secret_access_role

  create_secret_access_role_binding
}

main
