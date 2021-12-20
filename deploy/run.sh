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
    # Temporarily pull a branch that has Conjur:edge
    # due to a bug that is fixed but not yet released.
    # This will be removed once Conjur is released with the fix.
    # git clone git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID
    git clone --single-branch --branch downgrade-conjur-version git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    cmd="./start"
    if [ $CONJUR_DEPLOYMENT = "oss" ]; then
        cmd="$cmd --oss"
    fi
    cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd
  popd
}

main
