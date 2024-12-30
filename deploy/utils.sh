#!/bin/bash
set -euo pipefail

export KEY_VALUE_NOT_EXIST=" "
mkdir -p output

if [ "${PLATFORM}" = "kubernetes" ]; then
    cli_with_timeout="wait_for_it 300 kubectl"
    cli_without_timeout=kubectl
elif [ "${PLATFORM}" = "openshift" ]; then
    cli_with_timeout="wait_for_it 300 oc"
    cli_without_timeout=oc
fi

wait_for_it() {
  local timeout=$1
  local spacer=5
  shift
  if ! [ "${timeout}" = '-1' ]; then
    local times_to_run=$((timeout / spacer))

    for i in $(seq $times_to_run); do
      if cmd_output=$(eval "$@") ;
      then
        echo "$cmd_output"
        return 0
      fi
      sleep $spacer
    done

    # Last run evaluated. If this fails we return an error exit code to caller
    eval "$@"
  else
    echo "Waiting for '$*' forever"

    while ! eval "$@" > /dev/null; do
      sleep "${spacer}"
    done
    echo 'Success!'
  fi
}

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
  app_name=conjur-node
  if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
      app_name=conjur-oss
  fi
  get_pod_name "$CONJUR_NAMESPACE_NAME" app=$app_name,role=master
}

get_conjur_cli_pod_name() {
  get_pod_name "$CONJUR_NAMESPACE_NAME" 'app=conjur-cli'
}

runDockerCommand() {
  if [ "${PLATFORM}" = "kubernetes" ]; then
    docker run --rm \
      -i \
      -e UNIQUE_TEST_ID \
      -e CONJUR_APPLIANCE_IMAGE \
      -e CONJUR_LOG_LEVEL \
      -e CONJUR_FOLLOWER_COUNT \
      -e CONJUR_ACCOUNT \
      -e AUTHENTICATOR_ID \
      -e CONJUR_ADMIN_PASSWORD \
      -e DEPLOY_MASTER_CLUSTER \
      -e CONJUR_NAMESPACE_NAME \
      -e PLATFORM \
      -e TEST_PLATFORM \
      -e LOCAL_AUTHENTICATOR \
      -e APP_NAMESPACE_NAME \
      -e OPENSHIFT_URL \
      -e OPENSHIFT_VERSION \
      -e OPENSHIFT_USERNAME \
      -e OPENSHIFT_PASSWORD \
      -e DOCKER_REGISTRY_PATH \
      -e DOCKER_REGISTRY_URL \
      -e PULL_DOCKER_REGISTRY_PATH \
      -e PULL_DOCKER_REGISTRY_URL \
      -e GCLOUD_CLUSTER_NAME \
      -e GCLOUD_ZONE \
      -e GCLOUD_PROJECT_NAME \
      -e GCLOUD_SERVICE_KEY=/tmp$GCLOUD_SERVICE_KEY \
      -e MINIKUBE \
      -e MINISHIFT \
      -e DEV \
      -e TEST_NAME_PREFIX \
      -e CONJUR_DEPLOYMENT \
      -e RUN_IN_DOCKER \
      -e SUMMON_ENV \
      -e IMAGE_PULL_SECRET \
      -v $GCLOUD_SERVICE_KEY:/tmp$GCLOUD_SERVICE_KEY \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -v ~/.config:/root/.config \
      -v "$PWD/../helm":/helm \
      -v "$PWD/..":/src \
      -w /src/deploy \
      $TEST_RUNNER_IMAGE:$CONJUR_NAMESPACE_NAME \
      bash -c "
        ./platform_login.sh
        $1
      "
  else
    docker run --rm \
      -i \
      -e UNIQUE_TEST_ID \
      -e CONJUR_APPLIANCE_IMAGE \
      -e CONJUR_LOG_LEVEL \
      -e CONJUR_FOLLOWER_COUNT \
      -e CONJUR_ACCOUNT \
      -e AUTHENTICATOR_ID \
      -e CONJUR_ADMIN_PASSWORD \
      -e DEPLOY_MASTER_CLUSTER \
      -e CONJUR_NAMESPACE_NAME \
      -e PLATFORM \
      -e TEST_PLATFORM \
      -e LOCAL_AUTHENTICATOR \
      -e APP_NAMESPACE_NAME \
      -e OPENSHIFT_URL \
      -e OPENSHIFT_VERSION \
      -e OPENSHIFT_USERNAME \
      -e OPENSHIFT_PASSWORD \
      -e DOCKER_REGISTRY_PATH \
      -e DOCKER_REGISTRY_URL \
      -e PULL_DOCKER_REGISTRY_PATH \
      -e PULL_DOCKER_REGISTRY_URL \
      -e MINIKUBE \
      -e MINISHIFT \
      -e DEV \
      -e TEST_NAME_PREFIX \
      -e CONJUR_DEPLOYMENT \
      -e RUN_IN_DOCKER \
      -e SUMMON_ENV \
      -e IMAGE_PULL_SECRET \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -v "$PWD/../helm":/helm \
      -v "$PWD/..":/src \
      -w /src/deploy \
      $TEST_RUNNER_IMAGE:$CONJUR_NAMESPACE_NAME \
      bash -c "
        ./platform_login.sh
        $1
      "
  fi
}

configure_cli_pod() {
  announce "Configuring Conjur CLI."

  conjur_node_name="conjur-master"
  if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
      conjur_node_name="conjur-oss"
  fi
  conjur_url="https://$conjur_node_name.$CONJUR_NAMESPACE_NAME.svc.cluster.local"

  conjur_cli_pod=$(get_conjur_cli_pod_name)

  $cli_with_timeout "exec $conjur_cli_pod -- sh -c \"echo y | conjur init -a $CONJUR_ACCOUNT -u $conjur_url --self-signed --force\""

  $cli_with_timeout exec $conjur_cli_pod -- conjur login -i admin -p $CONJUR_ADMIN_PASSWORD
}

configure_conjur_url() {
  conjur_node_name="conjur-follower"
  if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
      conjur_node_name="conjur-oss"
  fi
  conjur_appliance_url=https://$conjur_node_name.$CONJUR_NAMESPACE_NAME.svc.cluster.local
  if [ "$CONJUR_DEPLOYMENT" = "dap" ]; then
      conjur_appliance_url="$conjur_appliance_url/api"
  fi
  conjur_authenticator_url=$conjur_appliance_url/authn-k8s/$AUTHENTICATOR_ID

  export CONJUR_APPLIANCE_URL=$conjur_appliance_url
  export CONJUR_AUTHN_URL=$conjur_authenticator_url
}

fetch_ssl_from_conjur() {
  selector="role=follower"
  cert_location="/opt/conjur/etc/ssl/conjur.pem"
  if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
    selector="app=conjur-cli"
    export cert_location="/home/cli/conjur-server.pem"
  fi

  export conjur_pod_name="$(get_pod_name "$CONJUR_NAMESPACE_NAME" "$selector")"
}

setup_helm_environment() {
  set_namespace $CONJUR_NAMESPACE_NAME

  configure_conjur_url

  ssl_location="conjur-server.pem"
  if [ "${DEV}" = "true" ]; then
    ssl_location="../conjur-server.pem"
  fi

  fetch_ssl_from_conjur
  # Save cert for later setting in Helm
  $cli_with_timeout "exec ${conjur_pod_name} --namespace $CONJUR_NAMESPACE_NAME cat $cert_location" > "$ssl_location"
  i=0
  while [[ ! -f "$ssl_location" || ! -s "$ssl_location" && $i -le 5 ]]; do
    i=$(( $i + 1 ))
    fetch_ssl_from_conjur
    # Save cert for later setting in Helm
    $cli_with_timeout "exec ${conjur_pod_name} --namespace $CONJUR_NAMESPACE_NAME cat $cert_location" > "$ssl_location"
  done

  set_namespace $APP_NAMESPACE_NAME
}

set_image_path() {
  image_path="$APP_NAMESPACE_NAME"
  if [[ "${PLATFORM}" = "openshift" && "${DEV}" = "false" ]]; then
    # Image path needs to point to internal registry path to access image
    image_path="${PULL_DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}"
  elif [[ "${PLATFORM}" = "kubernetes" && "${DEV}" = "false" ]]; then
    image_path="${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}"
  fi
  export image_path
}

fill_helm_chart() {
  helm_path="."
  if [ "${DEV}" = "false" ]; then
    helm_path=".."
  fi

  id=${1:-""}
  rm -rf "$helm_path/helm/secrets-provider/ci/${id}test-values-$UNIQUE_TEST_ID.yaml"

  set_image_path

  i=0
  while [[ ! -f "$helm_path/helm/secrets-provider/ci/${id}test-values-$UNIQUE_TEST_ID.yaml" && $i -le 5 ]]; do
    i=$(( $i + 1 ))
    sed -e "s#{{ SECRETS_PROVIDER_ROLE }}#${SECRETS_PROVIDER_ROLE:-"secrets-provider-role"}#g" \
      -e "s#{{ SECRETS_PROVIDER_ROLE_BINDING }}#${SECRETS_PROVIDER_ROLE_BINDING:-"secrets-provider-role-binding"}#g" \
      -e "s#{{ CREATE_SERVICE_ACCOUNT }}#${CREATE_SERVICE_ACCOUNT:-"true"}#g" \
      -e "s#{{ SERVICE_ACCOUNT }}#${SERVICE_ACCOUNT:-"secrets-provider-service-account"}#g" \
      -e "s#{{ K8S_SECRETS }}#${K8S_SECRETS:-"test-k8s-secret"}#g" \
      -e "s#{{ CONJUR_ACCOUNT }}#${CONJUR_ACCOUNT:-"cucumber"}#g" \
      -e "s#{{ CONJUR_APPLIANCE_URL }}#${CONJUR_APPLIANCE_URL:-"https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local/api"}#g" \
      -e "s#{{ CONJUR_AUTHN_URL }}#${CONJUR_AUTHN_URL:-"https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local/api/authn-k8s/${AUTHENTICATOR_ID}"}#g" \
      -e "s#{{ CONJUR_AUTHN_LOGIN }}# ${CONJUR_AUTHN_LOGIN:-"host/conjur/authn-k8s/${AUTHENTICATOR_ID}/apps/${APP_NAMESPACE_NAME}/*/*"}#g"  \
      -e "s#{{ SECRETS_PROVIDER_SSL_CONFIG_MAP }}# ${SECRETS_PROVIDER_SSL_CONFIG_MAP:-"secrets-provider-ssl-config-map"}#g" \
      -e "s#{{ IMAGE_PULL_POLICY }}# ${IMAGE_PULL_POLICY:-"IfNotPresent"}#g" \
      -e "s#{{ IMAGE }}# ${IMAGE:-"$image_path/secrets-provider"}#g" \
      -e "s#{{ TAG }}# ${TAG:-"latest"}#g" \
      -e "s#{{ LABELS }}# ${LABELS:-"app: test-helm"}#g" \
      -e "s#{{ DEBUG }}# ${DEBUG:-"false"}#g" \
      -e "s#{{ LOG_LEVEL }}# ${LOG_LEVEL:-"info"}#g" \
      -e "s#{{ RETRY_COUNT_LIMIT }}# ${RETRY_COUNT_LIMIT:-"5"}#g" \
      -e "s#{{ RETRY_INTERVAL_SEC }}# ${RETRY_INTERVAL_SEC:-"5"}#g" \
      -e "s#{{ IMAGE_PULL_SECRET }}# ${IMAGE_PULL_SECRET:-""}#g" \
      "$helm_path/helm/secrets-provider/ci/test-values-template.yaml" > "$helm_path/helm/secrets-provider/ci/${id}test-values-$UNIQUE_TEST_ID.yaml"
  done
}

fill_helm_chart_no_override_defaults() {
  rm -rf "../helm/secrets-provider/ci/take-default-test-values-$UNIQUE_TEST_ID.yaml"

  i=0
  while [[ ! -f "../helm/secrets-provider/ci/take-default-test-values-$UNIQUE_TEST_ID.yaml" && $i -le 5 ]]; do
    i=$(( $i + 1 ))
    sed -e "s#{{ K8S_SECRETS }}#${K8S_SECRETS}#g" \
      -e "s#{{ CONJUR_ACCOUNT }}#${CONJUR_ACCOUNT}#g" \
      -e "s#{{ LABELS }}# ${LABELS}#g" \
      -e "s#{{ CONJUR_APPLIANCE_URL }}#${CONJUR_APPLIANCE_URL}#g" \
      -e "s#{{ CONJUR_AUTHN_URL }}#${CONJUR_AUTHN_URL}#g" \
      -e "s#{{ CONJUR_AUTHN_LOGIN }}# ${CONJUR_AUTHN_LOGIN}#g"  \
      "../helm/secrets-provider/ci/take-default-test-values-template.yaml" > "../helm/secrets-provider/ci/take-default-test-values-$UNIQUE_TEST_ID.yaml"
  done
}

fill_helm_chart_test_image() {
  rm -rf "../helm/secrets-provider/ci/take-image-values-$UNIQUE_TEST_ID.yaml"

  i=0
  while [[ ! -f "../helm/secrets-provider/ci/take-image-values-$UNIQUE_TEST_ID.yaml" && $i -le 5 ]]; do
    i=$(( $i + 1 ))
    sed -e "s#{{ IMAGE }}#${IMAGE}#g" \
      -e "s#{{ TAG }}#${TAG}#g" \
      -e "s#{{ IMAGE_PULL_POLICY }}#${IMAGE_PULL_POLICY}#g" \
      "../helm/secrets-provider/ci/take-image-values-template.yaml" > "../helm/secrets-provider/ci/take-image-values-$UNIQUE_TEST_ID.yaml"
  done
}

deploy_chart() {
  pushd ../
    fill_helm_chart
    helm install -f "helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
      secrets-provider ./helm/secrets-provider \
      --set-file environment.conjur.sslCertificate.value="conjur-server.pem"
  popd
}

set_config_directory_path() {
  export DEV_CONFIG_DIR="dev/config/k8s"
  export CONFIG_DIR="config/k8s"
  if [[ "$PLATFORM" = "openshift" ]]; then
    export CONFIG_DIR="config/openshift"
  fi
}

deploy_helm_app() {
  set_config_directory_path

  helm_app_path="../$CONFIG_DIR/helm-app.yaml"
  if [ "${DEV}" = "true" ]; then
      helm_app_path="test/$CONFIG_DIR/helm-app.yaml"
  fi

  id=${1:-""}
  sed -e "s#{{ SERVICE_ACCOUNT }}#${SERVICE_ACCOUNT:-"secrets-provider-service-account"}#g" $helm_app_path |
  sed -e "s#{{ K8S_SECRET }}#${K8S_SECRET:-"test-k8s-secret"}#g" |
  sed -e "s#{{ ID }}#${id}#g" |
  $cli_with_timeout create -f -
}

create_k8s_role() {
  CONFIG_DIR="config/k8s"
  if [[ "$PLATFORM" = "openshift" ]]; then
    CONFIG_DIR="config/openshift"
  fi

  id=${1:-""}
  sed -e "s#{{ ID }}#${id}#g" "../$CONFIG_DIR/secrets-access-role.yaml" |
  sed -e "s#{{ APP_NAMESPACE_NAME }}#${APP_NAMESPACE_NAME}#g" |
  $cli_with_timeout create -f -
}

create_k8s_secret_for_helm_deployment() {
  helm_app_path="../config/k8s_secret.yml"
  if [ "${DEV}" = "true" ]; then
    helm_app_path="test/config/k8s_secret.yml"
  fi

  $cli_with_timeout create -f $helm_app_path
}

create_secret_access_role() {
  echo "Creating secrets access role"
  wait_for_it 600  "$CONFIG_DIR/secrets-access-role.sh.yml | $cli_without_timeout apply -f -"
}

create_secret_access_role_binding() {
  echo "Creating secrets access role binding"
  wait_for_it 600  "$CONFIG_DIR/secrets-access-role-binding.sh.yml | $cli_without_timeout apply -f -"
}

set_conjur_secret() {
  SECRET_NAME=$1
  SECRET_VALUE=$2
  echo "Set secret '$SECRET_NAME' to '$SECRET_VALUE'"
  set_namespace "$CONJUR_NAMESPACE_NAME"
  configure_cli_pod
  $cli_with_timeout "exec $(get_conjur_cli_pod_name) -- conjur variable set -i $SECRET_NAME -v \"$SECRET_VALUE\""
  set_namespace $APP_NAMESPACE_NAME
}

delete_test_secret() {
  load_policy "conjur-delete-secret"
}

restore_test_secret() {
  load_policy "conjur-secrets"
}

load_policy() {
  filename=$1
  set_namespace "$CONJUR_NAMESPACE_NAME"
  configure_cli_pod

  pushd "../../policy"
    mkdir -p ./generated
    ./templates/$filename.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.$filename.yml
  popd
  
  conjur_cli_pod=$(get_conjur_cli_pod_name)
  $cli_with_timeout "exec $conjur_cli_pod -- rm -rf /tmp/policy"
  $cli_with_timeout "cp ../../policy $conjur_cli_pod:/tmp/policy"

  $cli_with_timeout "exec $(get_conjur_cli_pod_name) -- \
    conjur policy update -b root -f \"/tmp/policy/generated/$APP_NAMESPACE_NAME.$filename.yml\""

  $cli_with_timeout "exec $conjur_cli_pod -- rm -rf /tmp/policy"

  set_namespace $APP_NAMESPACE_NAME
}

yaml_print_key_name_value() {
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

test_secret_is_provided() {
  secret_value=$1
  variable_name="${2:-secrets/test_secret}"
  environment_variable_name="${3:-TEST_SECRET}"

  set_namespace "$CONJUR_NAMESPACE_NAME"
  conjur_cli_pod=$(get_conjur_cli_pod_name)
  $cli_with_timeout "exec $conjur_cli_pod -- conjur variable set -i \"$variable_name\" -v $secret_value"

  set_namespace "$APP_NAMESPACE_NAME"
  deploy_env

  echo "Verifying pod test_env has environment variable '$environment_variable_name' with value '$secret_value'"
  pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"
  verify_secret_value_in_pod "$pod_name" "$environment_variable_name" "$secret_value"
}

verify_secret_value_in_pod() {
  pod_name=$1
  environment_variable_name=$2
  expected_value=$3

  if [[ $expected_value == "" ]] ; then
    # Ensure that the secret is empty by using 'grep -v .'
    expected_value="-v ."
  fi

  actual_value=$($cli_with_timeout "exec -n $APP_NAMESPACE_NAME ${pod_name} -- \
      printenv | grep $environment_variable_name | cut -d '=' -f 2-")
  echo "Actual value: $actual_value"
      
  $cli_with_timeout "exec -n $APP_NAMESPACE_NAME ${pod_name} -- \
    printenv | grep $environment_variable_name | cut -d '=' -f 2- | grep $expected_value"
}

get_app_logs_container() {
    echo "Get logs from the Secrets Provider container"
    set_namespace "$APP_NAMESPACE_NAME"
    helm=$(helm ls -aq)
    $cli_without_timeout get pods

    if [[ -z "$helm" ]]; then
      pod_name=$($cli_without_timeout get pods --selector app=test-env --no-headers  | awk '{print $1}' )
      echo "pod_name="$pod_name

      if [[ $pod_name != "" ]]; then
        $cli_without_timeout describe pod $pod_name
        $cli_without_timeout get events
        $cli_without_timeout logs $pod_name -c cyberark-secrets-provider-for-k8s > "output/$SUMMON_ENV-secrets-provider-logs.txt"
      fi
    else
      pod_name=$($cli_without_timeout get pods --selector app=test-helm --no-headers | awk '{print $1}' )
      echo "pod_name="$pod_name

      if [[ $pod_name != "" ]]; then
          $cli_without_timeout describe pod $pod_name
          $cli_without_timeout get events
         $cli_without_timeout logs $pod_name  > "output/$SUMMON_ENV-secrets-provider-logs-with-helm.txt"
      fi
    fi
}

get_conjur_logs_container() {
    echo "Get logs from the DAP Follower/Conjur container(s)"
    set_namespace "$CONJUR_NAMESPACE_NAME"
    selector="role=follower"

    if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
      selector="app=conjur-oss"
    fi

    $cli_without_timeout get pods
    $cli_with_timeout get pods --selector=$selector --no-headers
    pod_list=$($cli_without_timeout get pods --selector=$selector --no-headers | awk '{ print $1 }')

    if [[ -z "$pod_list" ]]; then
      echo "Pod doesn't exist. DAP Follower/Conjur logs were unable to be retrieved"
    else
      if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
        $cli_without_timeout logs $pod_list -c conjur > "output/$SUMMON_ENV-$pod_list-logs.txt"
      else
        #Fetch multiple container logs for case where there are more than one DAP Follower
        for pod in $pod_list
        do
          $cli_without_timeout logs $pod  > "output/$SUMMON_ENV-$pod-logs.txt"
        done
      fi
    fi
}

get_logs() {
    echo "Fetching all success / error logs"
    get_conjur_logs_container
    get_app_logs_container
}

# Return the pods information of the test-env without the headers
# For example: 'test-app-5-fab52b20-0 secret-provider-0 1/1 Running 1 20m'
get_pods_info() {
  $cli_with_timeout get pods \
    --namespace="$APP_NAMESPACE_NAME" \
    --selector app=test-env \
    --no-headers
}

# Return the pod name of a given namespace and app name
# For example: 'secret-provider-0'
get_pod_name() {
  local namespace=$1
  local selector=$2

  pod_name=$(
    $cli_with_timeout get pods \
      --namespace="${namespace}" \
      --selector "${selector}" \
      -o jsonpath='{.items[].metadata.name}'
  )

  if [[ -z $pod_name ]]; then
    echo "Unable to find ${selector} in namespace ${namespace} - aborting."
    $cli_with_timeout describe pods --namespace="${namespace}"
    exit 1
  fi

  echo "${pod_name}"
}

# Waits until the given job has completed its deployment
wait_for_job() {
  local job_name=$1

  echo "Waiting for job $job_name to complete"

  # we use $cli_without_timeout as we give the timeout as input to the 'wait' command
  # In case the job fails to complete we print the error
  if ! $cli_without_timeout wait --for=condition=complete --timeout=300s job/"$job_name" ; then
    pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" "job-name=$job_name")"
    echo "Printing details on the failing job"
    $cli_without_timeout describe pod "$pod_name"
    echo "Printing logs of the failing pod"
    $cli_without_timeout logs "$pod_name"
  fi
}

deploy_env() {
  export SECRETS_MODE=${SECRETS_MODE:-"k8s"}
  export TEMPLATE_OVERRIDE=${TEMPLATE_OVERRIDE:-""}
  local yaml_template_name="test-env"

  case $SECRETS_MODE in
    "k8s")
      if [[ "$DEV" = "true" ]]; then
        yaml_template_name="secrets-provider-init-container"
      else
        yaml_template_name="test-env"
      fi
      ;;
    "k8s-rotation")
      if [[ "$DEV" = "true" ]]; then
        yaml_template_name="secrets-provider-k8s-rotation"
      else
        yaml_template_name="test-env-k8s-rotation"
      fi
      ;;
    "p2f")
      if [[ "$DEV" = "true" ]]; then
        yaml_template_name="secrets-provider-init-push-to-file"
      else
        yaml_template_name="test-env-push-to-file"
      fi
      ;;
    "p2f-rotation")
      if [[ "$DEV" = "true" ]]; then
        yaml_template_name="secrets-provider-p2f-rotation"
      else
        yaml_template_name="test-env-p2f-rotation"
      fi
      ;;
    *)
      echo "Invalid or missing SECRETS_MODE variable. Allowed values are: k8s, k8s-rotation, p2f, p2f-rotation."
      echo "Deploying with default config (k8s)."
      ;;
  esac

  if [ ! -z "$TEMPLATE_OVERRIDE" ]
  then
    yaml_template_name="$TEMPLATE_OVERRIDE"
  fi

  generate_manifest_and_deploy $yaml_template_name
}

generate_manifest_and_deploy() {
  local yaml_template_name=$1
  local deployment_name="test-env"

  configure_conjur_url

  if [[ "$DEV" = "true" ]]; then
    mkdir -p $DEV_CONFIG_DIR/generated
    "$DEV_CONFIG_DIR/$yaml_template_name.sh.yml" > "$DEV_CONFIG_DIR/generated/$yaml_template_name.yml"
    $cli_with_timeout apply -f "$DEV_CONFIG_DIR/generated/$yaml_template_name.yml"

    $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=$deployment_name --no-headers | wc -l"
  else
    wait_for_it 600 "$CONFIG_DIR/$yaml_template_name.sh.yml | $cli_without_timeout apply -f -" || true

    expected_num_replicas=`$CONFIG_DIR/$yaml_template_name.sh.yml |  awk '/replicas:/ {print $2}' `
    
    # Deployment (Deployment for k8s and DeploymentConfig for Openshift) might fail on error flows, even before creating the pods. If so, re-deploy.
    $cli_with_timeout "get deployment $deployment_name -o jsonpath={.status.replicas} | grep '^${expected_num_replicas}$'|| $cli_without_timeout rollout latest deployment $deployment_name"

    echo "Expecting $expected_num_replicas deployed pods"
    $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=$deployment_name --no-headers | wc -l | grep $expected_num_replicas"
  fi
}
