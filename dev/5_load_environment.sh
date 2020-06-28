#!/bin/bash
set -euxo pipefail

. utils.sh

function main() {
  ./teardown_resources.sh

  set_namespace "$APP_NAMESPACE_NAME"

  docker_login

  configure_secret

  deploy_env

  verify_secret_in_env
}

function docker_login() {
  announce "Creating image pull secret."

  if [[ "${PLATFORM}" == "kubernetes" ]]; then
       $cli_with_timeout delete --ignore-not-found secret dockerpullsecret

       $cli_with_timeout create secret docker-registry dockerpullsecret \
        --docker-server=$DOCKER_REGISTRY_URL \
        --docker-username=_ \
        --docker-password=_ \
        --docker-email=_
  elif [[ "$PLATFORM" == "openshift" ]]; then
      $cli_with_timeout delete --ignore-not-found secrets dockerpullsecret

      # TODO: replace the following with `$cli create secret`
      $cli_with_timeout secrets new-dockercfg dockerpullsecret \
            --docker-server=${DOCKER_REGISTRY_PATH} \
            --docker-username=_ \
            --docker-password=$($cli_with_timeout whoami -t) \
            --docker-email=_

      $cli_with_timeout secrets add serviceaccount/default secrets/dockerpullsecret --for=pull
  fi
}

function configure_secret() {
  announce "Configuring K8s Secret and access."

  export ENV_DIR="$PWD/config/k8s"
  if [[ "$PLATFORM" = "openshift" ]]; then
      export ENV_DIR="$PWD/config/openshift"
  fi

  echo "Create secret k8s-secret"
  $cli_with_timeout create -f $ENV_DIR/k8s-secret.yml

  echo "Create secret k8s-secret"
  $cli_with_timeout create -f $ENV_DIR/k8s-secret-sigal.yml

  $cli_with_timeout create -f $ENV_DIR/k8s-secret-sigal3.yml

  create_secret_access_role

  create_secret_access_role_binding

  set_namespace $CONJUR_NAMESPACE_NAME

  set_secret "secrets/test_secret" "supersecret"
#  set_secret "secrets/sigal_secret" "sigalsecret"
}

function verify_secret_in_env() {
  announce "Verifying that K8s Secret is in Application container"

  # TODO change test-env
  echo "Verifying pod test_env has environment variable 'TEST_SECRET' with value 'supersecret'"
  pod_name=$($cli_with_timeout "get pods --selector=app=test-env --no-headers" | awk '{ print $1 }')

  $cli_with_timeout "exec $pod_name -- \
      printenv | grep "TEST_SECRET" | cut -d '=' -f 2- | grep "supersecret""
}

main
