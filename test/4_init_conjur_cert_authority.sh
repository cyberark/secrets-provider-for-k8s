#!/bin/bash
set -euo pipefail

. utils.sh

announce "Initializing Conjur certificate authority."

set_namespace $CONJUR_NAMESPACE_NAME

conjur_master=$(get_master_pod_name)

$cli exec $conjur_master -- chpst -u conjur conjur-plugin-service possum rake authn_k8s:ca_init["conjur/authn-k8s/$AUTHENTICATOR_ID"]

echo "Certificate authority initialized."
