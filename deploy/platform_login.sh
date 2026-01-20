#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

main() {
  # KinD runs locally (in Jenkins via docker socket) and does not require platform login.
  if [[ "${KIND}" = "true" ]]; then
    exit 0
  fi

  # Log in to platform
  if [[ "$PLATFORM" = "kubernetes" ]]; then
    if [[ -n "${GCLOUD_SERVICE_KEY:-}" && -f "${GCLOUD_SERVICE_KEY}" ]]; then
      gcloud auth activate-service-account \
        --key-file "${GCLOUD_SERVICE_KEY}"
    else
      echo "GCLOUD_SERVICE_KEY is not set or file not found; skipping gcloud auth"
    fi

    # Only call gcloud get-credentials if the required vars are set.
    if [[ -n "${GCLOUD_CLUSTER_NAME:-}" && -n "${GCLOUD_ZONE:-}" && -n "${GCLOUD_PROJECT_NAME:-}" ]]; then
      gcloud container clusters get-credentials \
        "$GCLOUD_CLUSTER_NAME" \
        --zone "$GCLOUD_ZONE" \
        --project "$GCLOUD_PROJECT_NAME"

      if [[ -n "${DOCKER_REGISTRY_PATH:-}" ]]; then
        docker login "$DOCKER_REGISTRY_PATH" \
          -u oauth2accesstoken \
          -p "$(gcloud auth print-access-token)"
      else
        echo "DOCKER_REGISTRY_PATH is not set; skipping docker login via gcloud"
      fi
    else
      echo "GCLOUD cluster info variables (GCLOUD_CLUSTER_NAME/GCLOUD_ZONE/GCLOUD_PROJECT_NAME) not set; skipping cluster credentials and docker login via gcloud"
    fi
  elif [[ "$PLATFORM" = "openshift" ]]; then
    # Some of the URLs do not have the port loaded in conjurops so we need to
    # add it manually
    if [[ -n "${OPENSHIFT_URL}" ]] && [[ ! "${OPENSHIFT_URL}" =~ :[[:digit:]] ]]; then
      OPENSHIFT_URL="${OPENSHIFT_URL}:8443"
    fi
    oc login "$OPENSHIFT_URL" \
      --username=$OPENSHIFT_USERNAME \
      --password=$OPENSHIFT_PASSWORD \
      --insecure-skip-tls-verify=true
    if [[ -n "${DOCKER_REGISTRY_PATH:-}" ]]; then
      docker login \
        -u _ -p "$(oc whoami -t)" \
        "$DOCKER_REGISTRY_PATH"
    else
      echo "DOCKER_REGISTRY_PATH is not set; skipping docker login via oc"
    fi
  fi
}

main
