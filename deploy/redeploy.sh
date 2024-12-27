#!/bin/bash
set -euxo pipefail

pushd $(dirname "${BASH_SOURCE[0]}")

# If DEV env variable isn't set, source the bootstrap file
if [[ -z "${DEV:-}" ]]; then
  source "../bootstrap.env"
fi

source "../deploy/utils.sh"

set_config_directory_path

../deploy/teardown_resources.sh

if [ "${DEV}" = "false" ]; then
  announce "Creating image pull secret."
  if [[ "${PLATFORM}" == "kubernetes" ]]; then
   $cli_with_timeout delete --ignore-not-found secret $IMAGE_PULL_SECRET

   $cli_with_timeout create secret docker-registry dockerpullsecret \
    --docker-server="${PULL_DOCKER_REGISTRY_URL}" \
    --docker-username=_ \
    --docker-password=_ \
    --docker-email=_
  elif [[ "$PLATFORM" == "openshift" ]]; then
    $cli_with_timeout delete --ignore-not-found secrets dockerpullsecret

    $cli_with_timeout create secret docker-registry $IMAGE_PULL_SECRET \
      --docker-server="${PULL_DOCKER_REGISTRY_PATH}" \
      --docker-username=_ \
      --docker-password=$($cli_with_timeout whoami -t) \
      --docker-email=_

    $cli_with_timeout secrets link serviceaccount/default dockerpullsecret --for=pull
  fi
fi

echo "Create secret k8s-secret"
$cli_with_timeout create -f "$CONFIG_DIR/k8s-secret.yml"

wait_for_it 600  "$CONFIG_DIR/secrets-access-role.sh.yml | $cli_without_timeout apply -f -"

wait_for_it 600  "$CONFIG_DIR/secrets-access-role-binding.sh.yml | $cli_without_timeout apply -f -"

selector="role=follower"
cert_location="/opt/conjur/etc/ssl/conjur.pem"
if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
    selector="app=conjur-cli"
    cert_location="/home/cli/conjur-server.pem"
fi
conjur_pod_name="$(get_pod_name "$CONJUR_NAMESPACE_NAME" "$selector")"
ssl_cert=$($cli_with_timeout "exec ${conjur_pod_name} --namespace $CONJUR_NAMESPACE_NAME -- cat $cert_location")

export CONJUR_SSL_CERTIFICATE=$ssl_cert

deploy_env

popd
