#!/bin/bash
set -xeuo pipefail

. utils.sh
printenv > /tmp/printenv_local.debug

main() {
  deployConjur
  ./run_with_summon.sh
}

deployConjur() {
  pushd ..
    git clone --single-branch --branch master \
        https://github.com/cyberark/kubernetes-conjur-deploy.git \
        kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    cmd="./start"
    if [ $CONJUR_DEPLOYMENT = "oss" ]; then
        cmd="$cmd --oss"
    fi
    cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd
  popd
}

main
