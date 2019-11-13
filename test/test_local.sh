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
    # TODO: change to master/v0.1 once it is merged
    git clone --single-branch --branch deploy-oss git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    cmd="./start"
    if [ $CONJUR_DEPLOYMENT == "dap" ]; then
        cmd="$cmd --dap"
    fi
    cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd
  popd
}

main
