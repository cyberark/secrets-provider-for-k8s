#!/bin/bash
set -xeuo pipefail

. utils.sh

# Clean up when script completes and fails
finish() {
  announce 'Wrapping up and removing test environment'

  # Stop the running processes
  runDockerCommand "
    ./stop && cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && ./stop
  "
  # Remove the deploy directory
  rm -rf "kubernetes-conjur-deploy-$UNIQUE_TEST_ID"
}
trap finish EXIT

main() {
  mkdir -p output #location where Secrets Provider/Conjur logs will be saved
  buildTestRunnerImage
  deployConjur
  deployTest
}

buildTestRunnerImage() {
  pushd ..
  docker build --tag $TEST_RUNNER_IMAGE:$CONJUR_NAMESPACE_NAME \
    --file Dockerfile.e2e \
    --build-arg OPENSHIFT_CLI_URL=$OPENSHIFT_CLI_URL \
    --build-arg KUBECTL_CLI_URL=$KUBECTL_CLI_URL \
    .
  popd
}

deployConjur() {
  # Prepare Docker images
  # This is done outside of the container to avoid authentication errors when pulling from the internal registry
  # from inside the container
  docker pull $CONJUR_APPLIANCE_IMAGE

  git clone --single-branch --branch master \
      https://github.com/cyberark/kubernetes-conjur-deploy.git \
      kubernetes-conjur-deploy-$UNIQUE_TEST_ID

  cmd="./start"
  if [ $CONJUR_DEPLOYMENT = "oss" ]; then
      cmd="$cmd --oss"
  fi
  runDockerCommand "cd ./kubernetes-conjur-deploy-$UNIQUE_TEST_ID && LOG_LEVEL=debug $cmd"
}

deployTest() {
  runDockerCommand "./run_with_summon.sh"
}

main
