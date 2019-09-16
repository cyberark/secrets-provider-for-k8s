#!/bin/bash
set -xeuo pipefail

./platform_login.sh

./0_deploy_conjur_on_kubernetes.sh

./1_check_dependencies.sh

./stop

./2_create_test_app_namespace.sh

if [[ "${DEPLOY_MASTER_CLUSTER}" = "true" ]]; then
  ./3_load_conjur_policies.sh
  ./4_init_conjur_cert_authority.sh
fi

./5_build_cyberark_secrets_provider_image.sh

./6_deploy_test_env.sh

exit_code=1
for n in {1..5}; do
  pod_name=$(oc get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')
  if [[ "$(oc logs $pod_name)" == "supersecret" ]]; then
    exit_code=0
    break
  else
    sleep 5
  fi
done

./stop
./kubernetes-conjur-deploy-"$UNIQUE_TEST_ID"/stop
rm -rf "kubernetes-conjur-deploy-$UNIQUE_TEST_ID"

exit $exit_code
