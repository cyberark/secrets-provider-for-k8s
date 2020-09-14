#!/bin/bash
set -euxo pipefail

echo "Creating secrets access role without 'get' permission"
export SECRET_CLUSTER_ROLE_VERBS_VALUE="[ \"get\" ]"
create_secret_access_role

create_secret_access_role_binding

deploy_init_env

echo "Expecting secrets provider to fail with error 'CSPFK005D Failed to update k8s secret. Reason:...'"
$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^1$'"
pod_name=$($cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers" | awk '{print $1}')

$cli_with_timeout "logs $pod_name -c cyberark-secrets-provider-for-k8s | grep CSPFK005D"
