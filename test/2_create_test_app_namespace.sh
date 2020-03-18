#!/bin/bash
set -euo pipefail

. utils.sh

announce "Creating Test App namespace."

if [[ $PLATFORM == openshift ]]; then
  $cli_with_timeout "login -u $OPENSHIFT_USERNAME"
fi

set_namespace default

if has_namespace "$TEST_APP_NAMESPACE_NAME"; then
  echo "Namespace '$TEST_APP_NAMESPACE_NAME' exists, not going to create it."
  set_namespace $TEST_APP_NAMESPACE_NAME
else
  echo "Creating '$TEST_APP_NAMESPACE_NAME' namespace."

  if [ $PLATFORM = "kubernetes" ]; then
    $cli_with_timeout "create namespace $TEST_APP_NAMESPACE_NAME"
  elif [ $PLATFORM = "openshift" ]; then
    $cli_with_timeout "new-project $TEST_APP_NAMESPACE_NAME"
  fi

  set_namespace $TEST_APP_NAMESPACE_NAME
fi

$cli_with_timeout delete --ignore-not-found rolebinding test-app-conjur-authenticator-role-binding-$CONJUR_NAMESPACE_NAME

TEST_DIR="config/k8s"
if [[ "$PLATFORM" = "openshift" ]]; then
    TEST_DIR="config/openshift"
fi

./$TEST_DIR/test-app-conjur-authenticator-role-binding.sh.yml | $cli_with_timeout "create -f -"

if [[ $PLATFORM == openshift ]]; then
  # add permissions for Conjur admin user
  $cli_with_timeout "adm policy add-role-to-user system:registry $OPENSHIFT_USERNAME"
  $cli_with_timeout "adm policy add-role-to-user system:image-builder $OPENSHIFT_USERNAME"

  $cli_with_timeout "adm policy add-role-to-user admin $OPENSHIFT_USERNAME -n default"
  $cli_with_timeout "adm policy add-role-to-user admin $OPENSHIFT_USERNAME -n $TEST_APP_NAMESPACE_NAME"
  echo "Logging in as Conjur Openshift admin. Provide password as needed."
  $cli_with_timeout "login -u $OPENSHIFT_USERNAME"
fi
