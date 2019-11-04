#!/bin/bash
set -euxo pipefail

# TODO: replace the following with `$cli create secret`
$cli secrets new-dockercfg dockerpullsecret \
      --docker-server=${DOCKER_REGISTRY_PATH} \
      --docker-username=_ \
      --docker-password=$($cli whoami -t) \
      --docker-email=_

$cli secrets add serviceaccount/default secrets/dockerpullsecret --for=pull

echo "Create secret k8s-secret"
$cli create -f $TEST_CASES_K8S_CONFIG_DIR/k8s-secret.yml
