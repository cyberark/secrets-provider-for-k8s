#!/bin/bash
set -euxo pipefail

announce "Creating image pull secret."

if [[ "${PLATFORM}" == "kubernetes" ]]; then
     $cli delete --ignore-not-found secret dockerpullsecret

     $cli create secret docker-registry dockerpullsecret \
      --docker-server=$DOCKER_REGISTRY_URL \
      --docker-username=$DOCKER_USERNAME \
      --docker-password=$DOCKER_PASSWORD \
      --docker-email=_

elif [[ "$PLATFORM" == "openshift" ]]; then

    $cli delete --ignore-not-found secrets dockerpullsecret

    # TODO: replace the following with `$cli create secret`
    $cli secrets new-dockercfg dockerpullsecret \
          --docker-server=${DOCKER_REGISTRY_PATH} \
          --docker-username=_ \
          --docker-password=$($cli whoami -t) \
          --docker-email=_

    $cli secrets add serviceaccount/default secrets/dockerpullsecret --for=pull

    echo "Create secret k8s-secret"
fi

$cli create -f $TEST_CASES_K8S_CONFIG_DIR/k8s-secret.yml
