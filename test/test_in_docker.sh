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
    # taking v0.1 since latest kubernetes-conjur-deploy is not stable
    git clone --single-branch \
      --branch $KUBERNETES_CONJUR_DEPLOY_BRANCH \
      https://github.com/cyberark/kubernetes-conjur-deploy.git \
      kubernetes-conjur-deploy-$UNIQUE_TEST_ID
  popd

  runDockerCommand "cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && ./start"
}

function deployTest() {
  runDockerCommand "cd test && ./test_with_summon.sh"
}

main
