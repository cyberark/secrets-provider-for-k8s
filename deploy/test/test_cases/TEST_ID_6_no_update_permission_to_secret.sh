#!/bin/bash
set -euxo pipefail

echo "Creating secrets access role without 'get' permission"
export SECRET_CLUSTER_ROLE_VERBS_VALUE="[ \"get\" ]"
create_secret_access_role

create_secret_access_role_binding

deploy_init_env

echo "Expecting secrets provider to fail with error 'CSPFK005D Failed to update k8s secret. Reason:...'"
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK005D"
