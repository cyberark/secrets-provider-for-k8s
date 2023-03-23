#!/bin/bash
set -euxo pipefail

echo "Deploying Push to file tests"

echo "Deploying test_env without CONTAINER_MODE environment variable"
export CONTAINER_MODE_KEY_VALUE=$KEY_VALUE_NOT_EXIST

echo "Running Deployment push to file"
export SECRETS_MODE="p2f"
deploy_env

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
    content="$($cli_with_timeout exec "$pod_name" -c test-app -- cat /opt/secrets/conjur/"$f")"
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

