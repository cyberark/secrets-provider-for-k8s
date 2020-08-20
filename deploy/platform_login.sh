#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

main() {
  # Log in to platform
  if [[ "$PLATFORM" = "kubernetes" ]]; then
    gcloud auth activate-service-account \
      --key-file $GCLOUD_SERVICE_KEY
    gcloud container clusters get-credentials \
      $GCLOUD_CLUSTER_NAME \
      --zone $GCLOUD_ZONE \
      --project $GCLOUD_PROJECT_NAME
    docker login $DOCKER_REGISTRY_PATH \
      -u oauth2accesstoken \
      -p "$(gcloud auth print-access-token)"
  elif [[ "$PLATFORM" = "openshift" ]]; then
    # sensible default for OPENSHIFT_URL port
    if [[ -n "${OPENSHIFT_URL}" ]] && [[ "${OPENSHIFT_URL}" != *: ]]; then
     OPENSHIFT_URL="${OPENSHIFT_URL}:8443"
    fi

    oc login "$OPENSHIFT_URL" \
      --username=$OPENSHIFT_USERNAME \
      --password=$OPENSHIFT_PASSWORD \
      --insecure-skip-tls-verify=true
    docker login \
      -u _ -p "$(oc whoami -t)" \
      $DOCKER_REGISTRY_PATH
  fi
}

main
