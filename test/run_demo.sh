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

  docker tag "cyberark-secrets-provider-for-k8s:dev" "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"
  docker push "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"

  set_namespace $TEST_APP_NAMESPACE_NAME
  enableImagePull

  provideSecretAccessToServiceAccount

  $cli create -f demo/k8s-secret.yml

  deployDemoEnv

  $cli expose service/pet-store-env
}

function CreatePetStoreApp() {
  pushd .
    git clone --single-branch --branch master git@github.com:conjurdemos/pet-store-demo pet-store-demo-$UNIQUE_TEST_ID

    pushd pet-store-demo-$UNIQUE_TEST_ID
      ./bin/build
      readonly IMAGE_TAG="$(cat VERSION)"
    popd

    $cli new-project "${TEST_APP_NAMESPACE_NAME}"
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
  ./k8s-config/secrets-access-role.sh.yml | $cli create -f -

  ./k8s-config/secrets-access-role-binding.sh.yml | $cli create -f -
}

function deployDemoEnv() {
  conjur_node_pod=$($cli get pod --namespace $CONJUR_NAMESPACE_NAME --selector=app=conjur-node -o=jsonpath='{.items[].metadata.name}')

  # this variable is consumed in pet-store-env.sh.yml
  export CONJUR_SSL_CERTIFICATE=$($cli exec --namespace $CONJUR_NAMESPACE_NAME "${conjur_node_pod}" cat /opt/conjur/etc/ssl/conjur-master.pem)

  ./demo/pet-store-env.sh.yml | $cli create -f -
}

main
