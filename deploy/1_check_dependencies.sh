#!/bin/bash
set -eo pipefail

. utils.sh

check_env_var "UNIQUE_TEST_ID"
check_env_var "TEST_PLATFORM"

check_env_var "CONJUR_APPLIANCE_IMAGE"
check_env_var "CONJUR_FOLLOWER_COUNT"
check_env_var "CONJUR_ACCOUNT"
check_env_var "AUTHENTICATOR_ID"
check_env_var "CONJUR_ADMIN_PASSWORD"
check_env_var "CONJUR_NAMESPACE_NAME"

check_env_var "APP_NAMESPACE_NAME"

if [[ "${DEV}" = "false" ]]; then
  check_env_var "DOCKER_REGISTRY_PATH"
  check_env_var "DOCKER_REGISTRY_URL"

  if [[ "$PLATFORM" = "openshift" ]]; then
    check_env_var "OPENSHIFT_USERNAME"
    check_env_var "OPENSHIFT_PASSWORD"
    check_env_var "OPENSHIFT_VERSION"
  elif [[ "$PLATFORM" = "kubernetes" ]]; then
    check_env_var "GCLOUD_SERVICE_KEY"
    check_env_var "GCLOUD_CLUSTER_NAME"
    check_env_var "GCLOUD_ZONE"
    check_env_var "GCLOUD_PROJECT_NAME"
  fi
fi
