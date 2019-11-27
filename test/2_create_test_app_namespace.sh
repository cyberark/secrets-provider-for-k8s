#!/bin/bash
set -euo pipefail

. utils.sh

announce "Creating Test App namespace."

if [[ $PLATFORM == openshift ]]; then
  oc login -u $OPENSHIFT_USERNAME
fi

set_namespace default

if has_namespace "$TEST_APP_NAMESPACE_NAME"; then
  echo "Namespace '$TEST_APP_NAMESPACE_NAME' exists, not going to create it."
  set_namespace $TEST_APP_NAMESPACE_NAME
else
  echo "Creating '$TEST_APP_NAMESPACE_NAME' namespace."

  if [ $PLATFORM = 'kubernetes' ]; then
    $cli create namespace $TEST_APP_NAMESPACE_NAME
  elif [ $PLATFORM = 'openshift' ]; then
    $cli new-project $TEST_APP_NAMESPACE_NAME
  fi

  set_namespace $TEST_APP_NAMESPACE_NAME
fi

$cli delete --ignore-not-found rolebinding test-app-conjur-authenticator-role-binding-$CONJUR_NAMESPACE_NAME

export TEST_DIR="k8s-config"
if [[ "$PLATFORM" = "openshift" ]]; then
    export TEST_DIR="openshift-config"
fi

./$TEST_DIR/test-app-conjur-authenticator-role-binding.sh.yml | $cli create -f -

if [[ $PLATFORM == openshift ]]; then
  # add permissions for Conjur admin user
  oc adm policy add-role-to-user system:registry $OPENSHIFT_USERNAME
  oc adm policy add-role-to-user system:image-builder $OPENSHIFT_USERNAME

  oc adm policy add-role-to-user admin $OPENSHIFT_USERNAME -n default
  oc adm policy add-role-to-user admin $OPENSHIFT_USERNAME -n $TEST_APP_NAMESPACE_NAME
  echo "Logging in as Conjur Openshift admin. Provide password as needed."
  oc login -u $OPENSHIFT_USERNAME
fi
