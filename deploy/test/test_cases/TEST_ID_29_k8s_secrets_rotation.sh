#!/bin/bash
set -euxo pipefail

delete_test_secret() {
  load_policy "conjur-delete-secret"
}

restore_test_secret() {
  load_policy "conjur-secrets"
}

load_policy() {
  filename=$1
  set_namespace "$CONJUR_NAMESPACE_NAME"
  configure_cli_pod

  pushd "../../policy"
    mkdir -p ./generated
    ./templates/$filename.template.sh.yml > ./generated/$APP_NAMESPACE_NAME.$filename.yml
  popd
  
  conjur_cli_pod=$(get_conjur_cli_pod_name)
  $cli_with_timeout "exec $conjur_cli_pod -- rm -rf /policy"
  $cli_with_timeout "cp ../../policy $conjur_cli_pod:/policy"

  $cli_with_timeout "exec $(get_conjur_cli_pod_name) -- \
    conjur policy load --delete root \"/policy/generated/$APP_NAMESPACE_NAME.$filename.yml\""

  $cli_with_timeout "exec $conjur_cli_pod -- rm -rf ./policy"

  set_namespace $APP_NAMESPACE_NAME
}

create_secret_access_role

create_secret_access_role_binding

deploy_k8s_rotation_env

pod_name1="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

echo "Verify pod $pod_name1 has environment variable 'TEST_SECRET' with value 'supersecret'"
verify_secret_value_in_pod $pod_name1 TEST_SECRET supersecret

set_conjur_secret secrets/test_secret secret2
sleep 15

# Pod will be restarted by livenessProbe after secret changes. Get name of new pod.
pod_name1="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

verify_secret_value_in_pod $pod_name1 TEST_SECRET secret2

delete_test_secret

sleep 15

# Pod will be restarted by livenessProbe after secret changes. Get name of new pod.
pod_name1="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

verify_secret_value_in_pod $pod_name1 TEST_SECRET ""

# Restore the test secret to reset the environment
restore_test_secret
