#!/bin/bash
set -euo pipefail

. utils.sh

announce "Initializing Conjur certificate authority."

set_namespace $CONJUR_NAMESPACE_NAME

conjur_master=$(get_master_pod_name)

cmd='bundle exec rake authn_k8s:ca_init['"conjur/authn-k8s/$AUTHENTICATOR_ID"]
if [ "$CONJUR_DEPLOYMENT" = "dap" ]; then
  cmd='chpst -u api:conjur conjur-plugin-service possum '"$cmd"
fi
$cli_with_timeout "exec $conjur_master -- $cmd"

echo "Certificate authority initialized."
