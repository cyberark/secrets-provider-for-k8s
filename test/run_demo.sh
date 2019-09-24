#!/bin/bash
set -xeuo pipefail

. utils.sh

function main() {
  ./1_check_dependencies.sh

  deployConjur

  CreatePetStoreApp

  ./platform_login.sh

  ./stop

  ./2_create_test_app_namespace.sh

  if [[ "${DEPLOY_MASTER_CLUSTER}" = "true" ]]; then
    ./3_load_conjur_policies.sh
    ./4_init_conjur_cert_authority.sh
  fi

  # build cyberark-secrets-provider image
  pushd ..
    ./bin/build
  popd

  docker tag "cyberark-secrets-provider-for-k8s:dev" "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"
  docker push "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"

  set_namespace $TEST_APP_NAMESPACE_NAME
  enableImagePull

  provideSecretAccessToServiceAccount

  $cli create -f demo/k8s-secret.yml

  deployDemoEnv

  oc expose service/pet-store-env
}

function CreatePetStoreApp() {
  pushd .
    git clone --single-branch --branch master git@github.com:conjurdemos/pet-store-demo pet-store-demo-$UNIQUE_TEST_ID

    pushd pet-store-demo-$UNIQUE_TEST_ID
      ./bin/build
      readonly IMAGE_TAG="$(cat VERSION)"
    popd

    oc new-project "${TEST_APP_NAMESPACE_NAME}"
    docker tag "demo-app:${IMAGE_TAG}" "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/demo-app"
    docker push "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/demo-app"

    rm -rf pet-store-demo-$UNIQUE_TEST_ID
  popd
}

function deployConjur() {
  pushd ..
    git clone --single-branch --branch master git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    pushd kubernetes-conjur-deploy-$UNIQUE_TEST_ID
      ./start
    popd
  popd
}

function enableImagePull() {
  $cli delete secret dockerpullsecret --ignore-not-found=true
  # TODO: replace the following with `oc create secret`
  $cli secrets new-dockercfg dockerpullsecret \
        --docker-server=${DOCKER_REGISTRY_PATH} \
        --docker-username=_ \
        --docker-password=$($cli whoami -t) \
        --docker-email=_
  $cli secrets add serviceaccount/default secrets/dockerpullsecret --for=pull
}

function provideSecretAccessToServiceAccount() {
  $cli delete clusterrole secrets-access --ignore-not-found=true
  $cli create -f "k8s-config/secrets-access-role.yml"

  ./k8s-config/secrets-access-role-binding.yml.sh | $cli create -f -
}

function deployDemoEnv() {
  mkdir -p ./demo/generated
  ./demo/pet-store-env.yml.sh > ./demo/generated/pet-store-env.yml
  $cli exec "$($cli get pods --namespace $CONJUR_NAMESPACE_NAME | grep conjur-cluster -m 1 |  awk '{print $1}')" --namespace $CONJUR_NAMESPACE_NAME cat /opt/conjur/etc/ssl/conjur-master.pem  | while read i; do printf "    %19s\n" "$i"; done  >> demo/generated/pet-store-env.yml

  $cli create -f demo/generated/pet-store-env.yml
}

main
