#!/bin/bash
set -xeuo pipefail

. utils.sh

./platform_login.sh

./1_check_dependencies.sh

./stop

./2_create_test_app_namespace.sh

#if [[ "${DEPLOY_MASTER_CLUSTER}" = "true" ]]; then
#  ./3_load_conjur_policies.sh
#  ./4_init_conjur_cert_authority.sh
#fi

set_namespace $TEST_APP_NAMESPACE_NAME

echo "Publish docker image"
docker tag "secrets-provider-for-k8s:dev" \
         "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"
docker push "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"

readonly K8S_CONFIG_DIR="k8s-config"

selector="role=follower"
cert_location="/opt/conjur/etc/ssl/conjur.pem"
if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
    selector="app=conjur-cli"
    cert_location="/root/conjur-${CONJUR_ACCOUNT}.pem"
fi
conjur_pod_name=$($cli get pods \
                      --selector=$selector \
                      --namespace $CONJUR_NAMESPACE_NAME \
                      --no-headers | awk '{ print $1 }' | head -1)
ssl_cert=$($cli exec "${conjur_pod_name}" --namespace $CONJUR_NAMESPACE_NAME cat $cert_location)

export CONJUR_SSL_CERTIFICATE=$ssl_cert


pushd test_cases > /dev/null
  ./run_tests.sh
popd > /dev/null

./stop
../kubernetes-conjur-deploy-"d025879f-1"/stop
rm -rf "../kubernetes-conjur-deploy-d025879f-1"
