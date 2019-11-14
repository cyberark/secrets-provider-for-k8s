#!/bin/bash
set -xeuo pipefail

. utils.sh

./platform_login.sh

./1_check_dependencies.sh

./stop

./2_create_test_app_namespace.sh

if [[ "${DEPLOY_MASTER_CLUSTER}" = "true" ]]; then
  ./3_load_conjur_policies.sh
  ./4_init_conjur_cert_authority.sh
fi

set_namespace $TEST_APP_NAMESPACE_NAME

echo "Publish docker image"
docker tag "secrets-provider-for-k8s:dev" \
         "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"
docker push "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"

readonly K8S_CONFIG_DIR="k8s-config"

# this variable is consumed in test-env.sh.yml
conjur_node_pod=$($cli get pod --namespace $CONJUR_NAMESPACE_NAME \
                      --selector=app=conjur-node \
                      -o=jsonpath='{.items[].metadata.name}')
export CONJUR_SSL_CERTIFICATE=$($cli exec --namespace $CONJUR_NAMESPACE_NAME \
                                 "${conjur_node_pod}" \
                                  cat /opt/conjur/etc/ssl/conjur.pem)

pushd test_cases > /dev/null
  ./run_tests.sh
popd > /dev/null

./stop
../kubernetes-conjur-deploy-"$UNIQUE_TEST_ID"/stop
rm -rf "../kubernetes-conjur-deploy-$UNIQUE_TEST_ID"
