#!/bin/bash
set -euxo pipefail

deploy_push_to_file() {
  configure_conjur_url

  echo "Running Deployment push to file"

  if [[ "$DEV" = "true" ]]; then
    ./dev/config/k8s/secrets-provider-init-push-to-file.sh.yml > ./dev/config/k8s/secrets-provider-init-push-to-file.yml
    $cli_with_timeout apply -f ./dev/config/k8s/secrets-provider-init-push-to-file.yml

    $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=init-env --no-headers | wc -l"
  else
    wait_for_it 600 "$CONFIG_DIR/test-env-push-to-file.sh.yml | $cli_without_timeout apply -f -" || true

    expected_num_replicas=`$CONFIG_DIR/test-env-push-to-file.sh.yml |  awk '/replicas:/ {print $2}' `

    # Deployment (Deployment for k8s and DeploymentConfig for Openshift) might fail on error flows, even before creating the pods. If so, re-deploy.
    if [[ "$PLATFORM" = "kubernetes" ]]; then
        $cli_with_timeout "get deployment test-env -o jsonpath={.status.replicas} | grep '^${expected_num_replicas}$'|| $cli_without_timeout rollout latest deployment test-env"
    elif [[ "$PLATFORM" = "openshift" ]]; then
        $cli_with_timeout "get dc/test-env -o jsonpath={.status.replicas} | grep '^${expected_num_replicas}$'|| $cli_without_timeout rollout latest dc/test-env"
    fi

    echo "Expecting for $expected_num_replicas deployed pods"
    $cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-env --no-headers | wc -l | grep $expected_num_replicas"
  fi
}
echo "Deploying Push to file tests"

echo "Deploying test_env without CONTAINER_MODE environment variable"
export CONTAINER_MODE_KEY_VALUE=$KEY_VALUE_NOT_EXIST
deploy_push_to_file

echo "Expecting secrets provider to succeed as an init container"

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

$cli_with_timeout "get pod $pod_name --namespace=$APP_NAMESPACE_NAME | grep -c 1/1"

FILES="group1.yaml group2.json some-dotenv.env group4.bash group5.template"

declare -A expected_content
expected_content[group1.yaml]='"url": "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
"username": "some-user"
"password": "7H1SiSmYp@5Sw0rd"'
expected_content[group2.json]='{"url":"postgresql://test-app-backend.app-test.svc.cluster.local:5432","username":"some-user","password":"7H1SiSmYp@5Sw0rd"}'
expected_content[some-dotenv.env]='url="postgresql://test-app-backend.app-test.svc.cluster.local:5432"
username="some-user"
password="7H1SiSmYp@5Sw0rd"'
expected_content[group4.bash]='export url="postgresql://test-app-backend.app-test.svc.cluster.local:5432"
export username="some-user"
export password="7H1SiSmYp@5Sw0rd"'
expected_content[group5.template]='username | some-user
password | 7H1SiSmYp@5Sw0rd'

declare -A file_format
file_format[group1.yaml]="yaml"
file_format[group2.json]="json"
file_format[some-dotenv.env]="dotenv"
file_format[group4.bash]="bash"
file_format[group5.template]="template"

test_failed=false
for f in $FILES; do
    format="${file_format[$f]}"
    echo "Checking file $f content, file format: $format"
    content="$($cli_with_timeout exec "$pod_name" -- cat /opt/secrets/conjur/"$f")"
    if [ "$content" == "${expected_content[$f]}" ]; then
        echo "Push to File PASSED for $format!"
    else
        echo "Push to File FAILED for file format $format!"
        echo "Expected content:"
        echo "================="
        echo "${expected_content[$f]}"
        echo
        echo "Actual content:"
        echo "==============="
        echo "$content"
        echo
        test_failed=true
    fi
done
if "$test_failed"; then
    exit 1
fi

