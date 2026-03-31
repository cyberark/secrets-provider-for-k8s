#!/bin/bash
set -xeuo pipefail

. utils.sh
. kind_helpers.sh
printenv > /tmp/printenv_local.debug

main() {
  export KIND_CLUSTER_NAME="secrets-provider-dev"
  # Allow skipping cluster creation for diagnostics collection in existing cluster
  if [[ "${SKIP_KIND_CREATE:-false}" != "true" ]]; then
    kind_create_cluster "${KIND_CLUSTER_NAME}"
  else
    echo "SKIP_KIND_CREATE=true; connecting to existing cluster ${KIND_CLUSTER_NAME}"
  fi
  if [ "$CONJUR_DEPLOYMENT" != "cloud" ]; then
    deployConjur
  elif [[ "${DEV}" = "true" ]]; then
    cloud_login
  fi
  ./run_with_summon.sh
}

deployConjur() {
  pushd ..
    git clone --single-branch --branch master \
        https://github.com/cyberark/kubernetes-conjur-deploy.git \
        kubernetes-conjur-deploy-$UNIQUE_TEST_ID || true

    cmd="./start"
    if [ $CONJUR_DEPLOYMENT = "oss" ]; then
        cmd="$cmd --oss"
    fi
    cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd
  popd
}

main
