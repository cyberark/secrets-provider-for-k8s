#!/bin/bash
set -euxo pipefail

echo "Deploying Secrets rotation tests"
set_conjur_secret secrets/test_secret secret1

echo "Deploying test_env without CONTAINER_MODE environment variable"
export CONTAINER_MODE_KEY_VALUE=$KEY_VALUE_NOT_EXIST

echo "Running Deployment secrets rotation"

deploy_push_to_file "secrets-provider-secrets-rotation" "test-env-secrets-rotation"

echo "Expecting secrets provider to succeed as a sidecar container"

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

# Expect 2 continers since we're using a sidecar
$cli_with_timeout "get pod $pod_name --namespace=$APP_NAMESPACE_NAME | grep -c 2/2"

# Change a conjur variable
set_conjur_secret secrets/test_secret secret2

# Check if the new value is picked up by secrets provider
sleep 10

FILES="group1.yaml group2.json some-dotenv.env group4.bash group5.template"

declare -A expected_content
expected_content[group1.yaml]='"url": "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
"username": "some-user"
"password": "7H1SiSmYp@5Sw0rd"
"test": "secret2"'
expected_content[group2.json]='{"url":"postgresql://test-app-backend.app-test.svc.cluster.local:5432","username":"some-user","password":"7H1SiSmYp@5Sw0rd","test":"secret2"}'
expected_content[some-dotenv.env]='url="postgresql://test-app-backend.app-test.svc.cluster.local:5432"
username="some-user"
password="7H1SiSmYp@5Sw0rd"
test="secret2"'
expected_content[group4.bash]='export url="postgresql://test-app-backend.app-test.svc.cluster.local:5432"
export username="some-user"
export password="7H1SiSmYp@5Sw0rd"
export test="secret2"'
expected_content[group5.template]='username | some-user
password | 7H1SiSmYp@5Sw0rd
test | secret2'

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
        echo "Secrets Rotation PASSED for $format!"
    else
        echo "Secrets Rotation FAILED for file format $format!"
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

