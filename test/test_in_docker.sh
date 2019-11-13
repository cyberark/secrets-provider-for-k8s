#!/bin/bash
set -xeuo pipefail

. utils.sh

function main() {
  buildTestRunnerImage
  deployConjur
  deployTest
}

function buildTestRunnerImage() {
  docker build --tag $TEST_RUNNER_IMAGE:$CONJUR_NAMESPACE_NAME \
    --file Dockerfile \
    --build-arg OPENSHIFT_CLI_URL=$OPENSHIFT_CLI_URL \
    --build-arg KUBECTL_CLI_URL=$KUBECTL_CLI_URL \
    .
}

function deployConjur() {
  pushd ..
    # TODO: change to master/v0.1 once deploy-oss is merged
    # taking v0.1 since latest kubernetes-conjur-deploy is not stable
    git clone --single-branch \
      --branch deploy-oss \
      git@github.com:cyberark/kubernetes-conjur-deploy \
      kubernetes-conjur-deploy-$UNIQUE_TEST_ID
  popd

  cmd="./start"
  if [ $CONJUR_DEPLOYMENT == "dap" ]; then
      cmd="$cmd --dap"
  fi
  runDockerCommand "cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd"
}

function deployTest() {
  runDockerCommand "cd test && ./test_with_summon.sh"
}

main
