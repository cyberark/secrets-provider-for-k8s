#!/bin/bash
set -xeuo pipefail

# Script for making it easy to make a change locally and redeploy
./bin/build

summon --environment oc311 -f ./summon/secrets.yml update

function update(){
  docker tag "secrets-provider-for-k8s:dev" \
           "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"
  docker push "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"

  #echo "Running Deployment Manifest"
  #wait_for_it 600 "$ENV_DIR/app-env.sh.yml | $cli_without_timeout apply -f -"

    echo "Running Deployment Manifest"
    wait_for_it 600 "$ENV_DIR/app-env.sh.yml | $cli_without_timeout apply -f -"

  echo "Running App deployment Manifest"
  wait_for_it 600 "$ENV_DIR/separate-pod-env.sh.yml | $cli_without_timeout apply -f -"

  $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-app --no-headers | wc -l"
}