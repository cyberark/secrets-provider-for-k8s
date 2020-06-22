#!/bin/bash
set -xeuo pipefail

. utils.sh

# Clean up when script completes and fails
function finish {
  announce 'Wrapping up and removing dev environment'

  # Stop the running processes
  ../stop
  ../kubernetes-conjur-deploy-"$UNIQUE_TEST_ID"/stop
  # Remove the deploy directory
  rm -rf "../kubernetes-conjur-deploy-$UNIQUE_TEST_ID"
}
trap finish EXIT

../platform_login.sh

../1_check_dependencies.sh

../2_create_app_namespace.sh

if [[ "${DEPLOY_MASTER_CLUSTER}" = "true" ]]; then
  ../3_load_conjur_policies.sh
  ../4_init_conjur_cert_authority.sh
fi

set_namespace $APP_NAMESPACE_NAME

echo "Publish docker image"
docker tag "secrets-provider-for-k8s:dev" \
         "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"
docker push "${DOCKER_REGISTRY_PATH}/${APP_NAMESPACE_NAME}/secrets-provider"

selector="role=follower"
cert_location="/opt/conjur/etc/ssl/conjur.pem"
if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
    selector="app=conjur-cli"
    cert_location="/root/conjur-${CONJUR_ACCOUNT}.pem"
fi
conjur_pod_name=$($cli_with_timeout get pods --selector=$selector --namespace $CONJUR_NAMESPACE_NAME --no-headers | awk '{ print $1 }' | head -1)
ssl_cert=$($cli_with_timeout "exec ${conjur_pod_name} --namespace $CONJUR_NAMESPACE_NAME cat $cert_location")

export CONJUR_SSL_CERTIFICATE=$ssl_cert

./5_load_environment.sh
