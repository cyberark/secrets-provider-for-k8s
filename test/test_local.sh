#!/bin/bash
set -xeuo pipefail

. utils.sh
printenv > /tmp/printenv_test_local.debug

function main() {
  deployConjur
  ./test_with_summon.sh
}

function deployConjur() {
  pushd ..
     # taking v0.1 since latest kubernetes-conjur-deploy is not stable
    git clone --single-branch \
       --branch $KUBERNETES_CONJUR_DEPLOY_BRANCH \
       https://github.com/cyberark/kubernetes-conjur-deploy.git \
       kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    pushd kubernetes-conjur-deploy-$UNIQUE_TEST_ID
      ./start
    popd
  popd
}

main
