#!/bin/bash -eu

. utils.sh

set_namespace $TEST_APP_NAMESPACE_NAME

echo "Publish docker image"
docker tag "cyberark-secrets-provider-for-k8s:dev" "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"
docker push "${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider"

echo "Enable image pull"
$cli delete secret dockerpullsecret --ignore-not-found=true
# TODO: replace the following with `oc create secret`
$cli secrets new-dockercfg dockerpullsecret \
      --docker-server=${DOCKER_REGISTRY_PATH} \
      --docker-username=_ \
      --docker-password=$($cli whoami -t) \
      --docker-email=_
$cli secrets add serviceaccount/default secrets/dockerpullsecret --for=pull

readonly K8S_CONFIG_DIR="k8s-config"

$cli delete clusterrole secrets-access --ignore-not-found=true
$cli create -f $K8S_CONFIG_DIR/secrets-access-role.yml

./$K8S_CONFIG_DIR/secrets-access-role-binding.yml.sh | $cli create -f -

conjur_node_pod=$($cli get pod --namespace $CONJUR_NAMESPACE_NAME --selector=app=conjur-node -o=jsonpath='{.items[].metadata.name}')

# this variable is consumed in test-env.yml.sh
export CONJUR_SSL_CERTIFICATE=$($cli exec --namespace $CONJUR_NAMESPACE_NAME "${conjur_node_pod}" cat /opt/conjur/etc/ssl/conjur-master.pem)

./$K8S_CONFIG_DIR/test-env.yml.sh | $cli create -f -

$cli create -f $K8S_CONFIG_DIR/k8s-secret.yml
