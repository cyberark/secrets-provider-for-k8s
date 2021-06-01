#!/bin/bash
set -euxo pipefail

if [ "${DEV}" = "false" ]; then
  announce "Creating image pull secret."
  if [[ "${PLATFORM}" == "kubernetes" ]]; then
   $cli_with_timeout delete --ignore-not-found secret dockerpullsecret

   $cli_with_timeout create secret docker-registry dockerpullsecret \
    --docker-server="${PULL_DOCKER_REGISTRY_URL}" \
    --docker-username=_ \
    --docker-password=_ \
    --docker-email=_
   echo "Create secret k8s-secret"
   $cli_with_timeout create -f $CONFIG_DIR/k8s-secret.yml
  fi
fi
