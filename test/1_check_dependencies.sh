#!/bin/bash
set -eo pipefail

. utils.sh

check_env_var "UNIQUE_TEST_ID"

check_env_var "CONJUR_VERSION"
check_env_var "CONJUR_APPLIANCE_IMAGE"
check_env_var "CONJUR_FOLLOWER_COUNT"
check_env_var "CONJUR_ACCOUNT"
check_env_var "AUTHENTICATOR_ID"
check_env_var "CONJUR_ADMIN_PASSWORD"
check_env_var "CONJUR_NAMESPACE_NAME"

check_env_var "TEST_APP_NAMESPACE_NAME"

check_env_var "DOCKER_REGISTRY_PATH"

if [[ "$PLATFORM" == "openshift" ]]; then
  check_env_var "OSHIFT_CONJUR_ADMIN_USERNAME"
  check_env_var "OSHIFT_CLUSTER_ADMIN_USERNAME"
  check_env_var "OPENSHIFT_USERNAME"
  check_env_var "OPENSHIFT_PASSWORD"
fi
