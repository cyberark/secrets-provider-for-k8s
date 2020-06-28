#!/bin/bash
set -xeuo pipefail

# lookup app-env.sh.yml for explanation.
export KEY_VALUE_NOT_EXIST=" "

wait_for_it() {
  local timeout=$1
  local spacer=2
  shift

  if ! [ $timeout = '-1' ]; then
    local times_to_run=$((timeout / spacer))

    #echo "Waiting for '$@' up to $timeout s"
    for i in $(seq $times_to_run); do
      eval $*  && return 0
      sleep $spacer
    done

    # Last run evaluated. If this fails we return an error exit code to caller
    eval $*
  else
    echo "Waiting for '$*' forever"

    while ! eval $* > /dev/null; do
      sleep $spacer
    done
  echo 'Success!'
  fi
}

if [ $PLATFORM = "kubernetes" ]; then
    cli_with_timeout="wait_for_it 600 kubectl"
    cli_without_timeout=kubectl
elif [ $PLATFORM = "openshift" ]; then
    cli_with_timeout="wait_for_it 600 oc"
    cli_without_timeout=oc
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
  # We don't need a timeout here as false is a valid output.
  # Running with a timeout will run this command repeatedly for no reason, ending with the same result
  if $cli_without_timeout get namespace  "$1" > /dev/null; then
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
  $cli_with_timeout config set-context "$($cli_with_timeout config current-context)" --namespace="$1" > /dev/null
}

get_master_pod_name() {
  app_name=conjur-cluster
  if [ "$CONJUR_DEPLOYMENT" = "dap" ]; then
      app_name=conjur-node
  fi
  pod_list=$($cli_with_timeout get pods --selector app=$app_name,role=master --no-headers | awk '{ print $1 }')
  echo $pod_list | awk '{print $1}'
}

get_conjur_cli_pod_name() {
  pod_list=$($cli_with_timeout get pods --selector app=conjur-cli --no-headers | awk '{ print $1 }')
  echo $pod_list | awk '{print $1}'
}

configure_cli_pod() {
  announce "Configuring Conjur CLI."

  conjur_node_name="conjur-cluster"
  if [ "$CONJUR_DEPLOYMENT" = "dap" ]; then
      conjur_node_name="conjur-master"
  fi
  conjur_url="https://$conjur_node_name.$CONJUR_NAMESPACE_NAME.svc.cluster.local"

  conjur_cli_pod=$(get_conjur_cli_pod_name)

  $cli_with_timeout "exec $conjur_cli_pod -- bash -c \"yes yes | conjur init -a $CONJUR_ACCOUNT -u $conjur_url\""

  $cli_with_timeout exec $conjur_cli_pod -- conjur authn login -u admin -p $CONJUR_ADMIN_PASSWORD
}

function deploy_env {
  conjur_node_name="conjur-cluster"
  if [ "$CONJUR_DEPLOYMENT" = "dap" ]; then
      conjur_node_name="conjur-follower"
  fi
  conjur_appliance_url=https://$conjur_node_name.$CONJUR_NAMESPACE_NAME.svc.cluster.local
  if [ "$CONJUR_DEPLOYMENT" = "dap" ]; then
    conjur_appliance_url="$conjur_appliance_url/api"
  fi
  conjur_authenticator_url=$conjur_appliance_url/authn-k8s/$AUTHENTICATOR_ID

  export CONJUR_APPLIANCE_URL=$conjur_appliance_url
  export CONJUR_AUTHN_URL=$conjur_authenticator_url

  echo "Running Init Secrets Provider Manifest"
  wait_for_it 600 "$ENV_DIR/init.sh.yml | $cli_without_timeout apply -f -"

#  echo "Running Deployment Manifest"
#  wait_for_it 600 "$ENV_DIR/app-env.sh.yml | $cli_without_timeout apply -f -"
#
#  echo "Running App deployment Manifest"
#  wait_for_it 600 "$ENV_DIR/separate-pod-env.sh.yml | $cli_without_timeout apply -f -"

  $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-app --no-headers | wc -l"
}

function create_secret_access_role () {
  echo "Creating secrets access role"
  wait_for_it 600  "$ENV_DIR/secrets-access-role.sh.yml | $cli_without_timeout apply -f -"
}

function create_secret_access_role_binding () {
  echo "Creating secrets access role binding"
  wait_for_it 600  "$ENV_DIR/secrets-access-role-binding.sh.yml | $cli_without_timeout apply -f -"
}

function set_secret () {
  SECRET_NAME=$1
  SECRET_VALUE=$2
  echo "Set secret '$SECRET_NAME' to '$SECRET_VALUE'"
  set_namespace "$CONJUR_NAMESPACE_NAME"
  configure_cli_pod
  $cli_with_timeout "exec $(get_conjur_cli_pod_name) -- conjur variable values add $SECRET_NAME $SECRET_VALUE"
  set_namespace $APP_NAMESPACE_NAME
}

yaml_print_key_name_value () {
  spaces=$1
  key_name=${2:-""}
  key_value=${3:-""}

  if [ -z "$key_name" ]
  then
     echo ""
  else
    printf "$spaces- name: $key_name\n"
    if [ -z "$key_value" ]
    then
       echo ""
    else
       echo "$spaces  value: $key_value"
    fi
  fi
}

cli_get_pods_test_env () {
  $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers"
}

test_secret_is_provided () {
  secret_value=$1

  set_namespace "$CONJUR_NAMESPACE_NAME"
  conjur_cli_pod=$(get_conjur_cli_pod_name)
  $cli_with_timeout "exec $conjur_cli_pod -- conjur variable values add secrets/test_secret $secret_value"

  set_namespace "$APP_NAMESPACE_NAME"
  deploy_test_env

  echo "Verifying pod test_env has environment variable 'TEST_SECRET' with value '$secret_value'"
  pod_name=$(cli_get_pods_test_env | awk '{print $1}')
  verify_secret_value_in_pod "$pod_name" TEST_SECRET "$secret_value"
}