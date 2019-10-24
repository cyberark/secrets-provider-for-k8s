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
    git clone --single-branch --branch master git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    pushd kubernetes-conjur-deploy-$UNIQUE_TEST_ID
      ./start
    popd
  popd
}

main
