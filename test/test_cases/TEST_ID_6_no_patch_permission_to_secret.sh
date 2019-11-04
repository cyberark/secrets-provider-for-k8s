#!/bin/bash
set -euxo pipefail

echo "Creating secrets access role without 'get' permission"
export SECRET_CLUSTER_ROLE_VERBS_VALUE="[ \"get\" ]"
create_secret_access_role

create_secret_access_role_binding

deploy_test_env

echo "Expecting secrets provider to fail with error 'CSPFK005D Failed to patch k8s secret. Reason:...'"
pod_name=$(cli_get_pods_test_env | awk '{print $1}')
wait_for_it 600 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CSPFK005D'"
