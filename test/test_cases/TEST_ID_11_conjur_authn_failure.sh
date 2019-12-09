#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

export CONJUR_AUTHN_LOGIN="host/some-policy/non-existing-namespace/*/*"

deploy_test_env

echo "Expecting secrets provider to fail with error CAKC015E Login failed"
pod_name=$(cli_get_pods_test_env | awk '{print $1}')
wait_for_it 600 "$cli logs $pod_name -c cyberark-secrets-provider | grep 'CAKC015E'"
