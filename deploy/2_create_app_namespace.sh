#!/bin/bash
set -euo pipefail

. utils.sh

announce "Creating Application namespace."

if [[ $PLATFORM == openshift && "${DEV}" = "false" ]]; then
  $cli_with_timeout "login -u $OPENSHIFT_USERNAME -p $OPENSHIFT_PASSWORD"
fi

if has_namespace "$APP_NAMESPACE_NAME"; then
  echo "Namespace '$APP_NAMESPACE_NAME' exists, not going to create it."
  set_namespace $APP_NAMESPACE_NAME
else
  echo "Creating '$APP_NAMESPACE_NAME' namespace."

  if [ $PLATFORM = "kubernetes" ]; then
    $cli_with_timeout "create namespace $APP_NAMESPACE_NAME"
  elif [ $PLATFORM = "openshift" ]; then
    $cli_with_timeout "new-project $APP_NAMESPACE_NAME"
  fi

  set_namespace $APP_NAMESPACE_NAME
fi

$cli_with_timeout delete --ignore-not-found rolebinding app-conjur-authenticator-role-binding-$CONJUR_NAMESPACE_NAME

set_config_directory_path

wait_for_it 600  "./$CONFIG_DIR/app-conjur-authenticator-role-binding.sh.yml | $cli_without_timeout apply -f -"

if [[ $PLATFORM == openshift ]]; then
  # add permissions for Conjur admin user
  $cli_with_timeout "adm policy add-role-to-user system:registry $OPENSHIFT_USERNAME"
  $cli_with_timeout "adm policy add-role-to-user system:image-builder $OPENSHIFT_USERNAME"

  $cli_with_timeout "adm policy add-role-to-user admin $OPENSHIFT_USERNAME -n default"
  $cli_with_timeout "adm policy add-role-to-user admin $OPENSHIFT_USERNAME -n $APP_NAMESPACE_NAME"
  echo "Logging in as Conjur Openshift admin. Provide password as needed."
  $cli_with_timeout "login -u $OPENSHIFT_USERNAME -p $OPENSHIFT_PASSWORD"
fi
