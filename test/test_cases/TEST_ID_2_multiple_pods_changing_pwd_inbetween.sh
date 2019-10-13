#!/bin/bash
set -euxo pipefail

source $TEST_CASES_UTILS

echo "Creating secrets access role"
$TEST_CASES_K8S_CONFIG_DIR/secrets-access-role.sh.yml | $cli create -f -

echo "Creating secrets access role binding"
$TEST_CASES_K8S_CONFIG_DIR/secrets-access-role-binding.sh.yml | $cli create -f -

deploy_test_env

pod_name1=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'supersecret'"
wait_for_it 600 "$cli exec -n $TEST_APP_NAMESPACE_NAME ${pod_name1} printenv | grep TEST_SECRET | cut -d \"=\" -f 2 | grep 'supersecret'"

#echo "Modify secret test_secret to 'secret2'"
#set_namespace "$CONJUR_NAMESPACE_NAME"
#configure_cli_pod
#$cli exec $(get_conjur_cli_pod_name) -- conjur variable values add secrets/test_secret "secret2"
#set_namespace $TEST_APP_NAMESPACE_NAME
test_app_set_secret secrets/test_secret secret2


echo "Deleting pod $pod_name1"
$cli delete pod $pod_name1

pod_name2=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}')
echo "Verify pod $pod_name2 has environment variable 'TEST_SECRET' with value 'supersecret'"
wait_for_it 600 "$cli exec -n $TEST_APP_NAMESPACE_NAME ${pod_name2} printenv | grep TEST_SECRET | cut -d \"=\" -f 2 | grep 'secret2'"

#echo "Modify secret test_secret to 'secret3'"
#set_namespace "$CONJUR_NAMESPACE_NAME"
#configure_cli_pod
#$cli exec $(get_conjur_cli_pod_name) -- conjur variable values add secrets/test_secret "secret3"
#set_namespace $TEST_APP_NAMESPACE_NAME
test_app_set_secret secrets/test_secret secret3


echo "Setting deploymentconfig test-env to replicas"
$cli scale dc test-env --replicas=3

echo "Waiting for 3 running pod test-env"
wait_for_it 600 "$cli get pods | grep test-env | grep Running | wc -l | tr -d ' ' | grep '^3$'"

echo "Iterate over new pods and verify their secret was updated"
pod_names=$($cli get pods --namespace=$TEST_APP_NAMESPACE_NAME --selector app=test-env --no-headers | awk '{print $1}' | grep -v $pod_name2)
for new_pod in $pod_names
do
     echo "Verify pod $new_pod has environment variable 'TEST_SECRET' with value 'secret3'"
     wait_for_it 600 "$cli exec -n $TEST_APP_NAMESPACE_NAME ${new_pod} printenv | grep TEST_SECRET | cut -d \"=\" -f 2 | grep 'secret3'"
done
