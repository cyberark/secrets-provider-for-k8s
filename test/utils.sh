#!/bin/bash

if [ $PLATFORM = 'kubernetes' ]; then
    cli=kubectl
elif [ $PLATFORM = 'openshift' ]; then
    cli=oc
fi

check_env_var() {
  if [[ -z "${!1+x}" ]]; then
# where ${var+x} is a parameter expansion which evaluates to nothing if var is unset, and substitutes the string x otherwise.
# https://stackoverflow.com/questions/3601515/how-to-check-if-a-variable-is-set-in-bash/13864829#13864829
    echo "You must set $1 before running these scripts."
    exit 1
  fi
}

announce() {
  echo "++++++++++++++++++++++++++++++++++++++"
  echo ""
  echo "$@"
  echo ""
  echo "++++++++++++++++++++++++++++++++++++++"
}

has_namespace() {
  if $cli get namespace  "$1" > /dev/null; then
    true
  else
    false
  fi
}

set_namespace() {
  if [[ $# != 1 ]]; then
    printf "Error in %s/%s - expecting 1 arg.\n" "$(pwd)" $0
    exit 1
  fi

  $cli config set-context "$($cli config current-context)" --namespace="$1" > /dev/null
}

get_master_pod_name() {
  pod_list=$($cli get pods --selector app=conjur-node,role=master --no-headers | awk '{ print $1 }')
  echo $pod_list | awk '{print $1}'
}

get_conjur_cli_pod_name() {
  pod_list=$($cli get pods --selector app=conjur-cli --no-headers | awk '{ print $1 }')
  echo $pod_list | awk '{print $1}'
}

function runDockerCommand() {
  docker run --rm \
    -i \
    -e UNIQUE_TEST_ID \
    -e CONJUR_VERSION \
    -e CONJUR_APPLIANCE_IMAGE \
    -e CONJUR_FOLLOWER_COUNT \
    -e CONJUR_ACCOUNT \
    -e AUTHENTICATOR_ID \
    -e CONJUR_ADMIN_PASSWORD \
    -e DEPLOY_MASTER_CLUSTER \
    -e CONJUR_NAMESPACE_NAME \
    -e PLATFORM \
    -e LOCAL_AUTHENTICATOR \
    -e TEST_APP_NAMESPACE_NAME \
    -e OPENSHIFT_URL \
    -e OPENSHIFT_USERNAME \
    -e OPENSHIFT_PASSWORD \
    -e DOCKER_REGISTRY_PATH \
    -e MINIKUBE \
    -e MINISHIFT \
    -e CONJUR_VERSION \
    -e CONJUR_VERSION \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v ~/.config:/root/.config \
    -v ~/.docker:/root/.docker \
    -v "$PWD/..":/src \
    -w /src \
    $TEST_RUNNER_IMAGE:$CONJUR_NAMESPACE_NAME \
    bash -c "
      ./test/platform_login.sh
      $1
    "
}
