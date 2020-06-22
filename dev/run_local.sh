#!/bin/bash
set -xeuo pipefail

. utils.sh
printenv > /tmp/printenv_dev_local.debug

function main() {
  deployConjur
  ./run_with_summon.sh
}

function deployConjur() {
  pushd ..
    git clone --single-branch --branch deploy-oss-tag git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    cmd="./start"
    if [ $CONJUR_DEPLOYMENT == "dap" ]; then
        cmd="$cmd --dap"
    fi
    cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd
  popd
}

main
