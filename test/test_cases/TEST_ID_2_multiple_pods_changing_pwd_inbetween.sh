#!/bin/bash
set -euxo pipefail

echo "Creating secrets access role"
$TEST_CASES_K8S_CONFIG_DIR/secrets-access-role.sh.yml | $cli create -f -

echo "Creating secrets access role binding"
$TEST_CASES_K8S_CONFIG_DIR/secrets-access-role-binding.sh.yml | $cli create -f -

deploy_test_env

pod_name1=$(cli_get_pods_test_env | awk '{print $1}')

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'supersecret'"
verify_secret_value_in_pod $pod_name1 TEST_SECRET supersecret

test_app_set_secret secrets/test_secret secret2

echo "Deleting pod $pod_name1"
$cli delete pod $pod_name1

pod_name2=$(cli_get_pods_test_env | awk '{print $1}')
echo "Verify pod $pod_name2 has environment variable 'TEST_SECRET' with value 'supersecret'"
verify_secret_value_in_pod $pod_name2 TEST_SECRET secret2

test_app_set_secret secrets/test_secret secret3

echo "Setting deploymentconfig test-env to replicas"
$cli scale dc test-env --replicas=3

echo "Waiting for 3 running pod test-env"
wait_for_it 600 "$cli get pods | grep test-env | grep Running | wc -l | tr -d ' ' | grep '^3$'"

echo "Iterate over new pods and verify their secret was updated"
pod_names=$(cli_get_pods_test_env | awk '{print $1}' | grep -v $pod_name2)
for new_pod in $pod_names
do
     echo "Verify pod $new_pod has environment variable 'TEST_SECRET' with value 'secret3'"
     verify_secret_value_in_pod $new_pod TEST_SECRET secret3
done
