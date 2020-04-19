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
    git clone --single-branch --branch deploy-oss-merge-to-master git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    cmd="./start"
    if [ $CONJUR_DEPLOYMENT == "oss" ]; then
        cmd="$cmd --oss"
    fi
    cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd
  popd
}

main
