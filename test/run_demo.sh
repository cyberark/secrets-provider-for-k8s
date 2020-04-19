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

  docker tag "secrets-provider-for-k8s:dev" "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"
  docker push "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"

  set_namespace $TEST_APP_NAMESPACE_NAME
  enableImagePull

  provideSecretAccessToServiceAccount

  $cli_without_timeout create -f demo/k8s-secret.yml

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
    git clone --single-branch --branch deploy-oss-tag git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID

    cmd="./start"
    if [ $CONJUR_DEPLOYMENT == "dap" ]; then
        cmd="$cmd --dap"
    fi
    cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $cmd
  popd
}

function enableImagePull() {
  $cli_without_timeout delete secret dockerpullsecret --ignore-not-found=true
  # TODO: replace the following with `oc create secret`
  $cli_without_timeout secrets new-dockercfg dockerpullsecret \
        --docker-server=${DOCKER_REGISTRY_PATH} \
        --docker-username=_ \
        --docker-password=$($cli_without_timeout  whoami -t) \
        --docker-email=_
  $cli_without_timeout secrets add serviceaccount/default secrets/dockerpullsecret --for=pull
}

function provideSecretAccessToServiceAccount() {
  export TEST_CASES_DIR="$PWD/config/openshift"

  $cli_without_timeout delete clusterrole secrets-access-${UNIQUE_TEST_ID} --ignore-not-found=true

  pushd $TEST_CASES_DIR
    mkdir -p ./generated
  popd

  $TEST_CASES_DIR/secrets-access-role.sh.yml >  $TEST_CASES_DIR/generated/secrets-access-role.yml
  oc create -f $TEST_CASES_DIR/generated/secrets-access-role.yml

  $TEST_CASES_DIR/secrets-access-role-binding.sh.yml > $TEST_CASES_DIR/generated/secrets-access-role-binding.yml
  oc create -f $TEST_CASES_DIR/generated/secrets-access-role-binding.yml
}

function deployDemoEnv() {
  conjur_node_pod=$($cli_without_timeout get pod --namespace $CONJUR_NAMESPACE_NAME --selector=app=conjur-node -o=jsonpath='{.items[].metadata.name}')

  # this variable is consumed in pet-store-env.sh.yml
  export CONJUR_SSL_CERTIFICATE=$($cli_without_timeout  exec --namespace $CONJUR_NAMESPACE_NAME "${conjur_node_pod}" cat /opt/conjur/etc/ssl/conjur.pem)

  pushd demo
    mkdir -p ./generated
  popd

  demo/pet-store-env.sh.yml > demo/generated/pet-store-env.yml
  oc create -f demo/generated/pet-store-env.yml
}

main
