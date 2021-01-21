#!/bin/bash
set -euxo pipefail

echo "Creating secrets access role"
wait_for_it 600  "$CONFIG_DIR/secrets-access-role.sh.yml | $cli_without_timeout apply -f -"

echo "Creating secrets access role binding"
wait_for_it 600 "$CONFIG_DIR/secrets-access-role-binding.sh.yml | $cli_without_timeout apply -f -"

deploy_init_env

pod_name1="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'supersecret'"
verify_secret_value_in_pod $pod_name1 TEST_SECRET supersecret

echo "Deleting pod $pod_name1"
if [[ "$PLATFORM" = "kubernetes" ]]; then
    # Using scaling down instead of delete because in GKE, deleting a pod automatically triggers the spin up of another one
    # and therefore two pods were begin returned when parsing the namespace which messes up the test. While with scale down,
    # we can are able to successfully delete the pods and only deploy a new one, when we scale back up.
    $cli_with_timeout "scale deployment test-env --replicas=0"

    # Waiting until pod is successfully removed from the namespace before advancing.
    $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | tr -d ' ' | grep '^0$'"

    set_conjur_secret secrets/test_secret secret2

    $cli_with_timeout "scale deployment test-env --replicas=1"
elif [ $PLATFORM = "openshift" ]; then
    set_conjur_secret secrets/test_secret secret2

    $cli_with_timeout "delete pod $pod_name1"
fi

pod_name2="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

echo "Verify pod $pod_name2 has environment variable 'TEST_SECRET' with value 'secret2'"
verify_secret_value_in_pod $pod_name2 TEST_SECRET secret2

set_conjur_secret secrets/test_secret secret3

if [[ "$PLATFORM" = "kubernetes" ]]; then
    echo "Setting deployment test-env to replicas"
    $cli_with_timeout "scale deployment test-env --replicas=3"
elif [ $PLATFORM = "openshift" ]; then
    echo "Setting deploymentconfig test-env to replicas"
    $cli_with_timeout "scale dc test-env --replicas=3"
fi

echo "Waiting for 3 running pod test-env"
$cli_with_timeout "get pods | grep test-env | grep Running | wc -l | tr -d ' ' | grep '^3$'"

echo "Iterate over new pods and verify their secret was updated"
pod_names=$(get_pods_info | awk '{print $1}' | grep -v $pod_name2)
for new_pod in $pod_names
do
   echo "Verify pod $new_pod has environment variable 'TEST_SECRET' with value 'secret3'"
   verify_secret_value_in_pod $new_pod TEST_SECRET secret3
done
