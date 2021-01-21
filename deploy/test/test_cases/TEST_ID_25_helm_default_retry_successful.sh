#!/bin/bash
set -euxo pipefail

# This test verifies that default values for retry mechanism work
setup_helm_environment

pushd ../../
  set_image_path
  export IMAGE="$image_path/secrets-provider"
  export IMAGE_PULL_POLICY="IfNotPresent"
  export TAG="latest"
  export LABELS="app: test-helm"
  export DEBUG="true"
  export K8S_SECRETS="test-k8s-secret"
  export CONJUR_ACCOUNT="cucumber"
  export CONJUR_APPLIANCE_URL="https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local"
  export CONJUR_AUTHN_LOGIN="host/conjur/authn-k8s/${AUTHENTICATOR_ID}/apps/${APP_NAMESPACE_NAME}/*/*"
  # A parameter that will force a failure
  export CONJUR_AUTHN_URL="https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local/api/authn-k8s/${AUTHENTICATOR_ID}xyz"  # Configure retry mechanism with overriding defaults

  export RETRY_COUNT_LIMIT="1"
  export RETRY_INTERVAL_SEC="5"

  DEFAULT_RETRY_INTERVAL_SEC=1
  DEFAULT_RETRY_COUNT_LIMIT=5

  fill_helm_chart_test_image

  fill_helm_chart_no_override_defaults

  helm install -f "../helm/secrets-provider/ci/take-default-test-values-$UNIQUE_TEST_ID.yaml" \
    -f "../helm/secrets-provider/ci/take-image-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-helm')"

# Find initial authentication error that should trigger the retry
$cli_with_timeout "logs $pod_name | grep 'CSPFK010E Failed to authenticate'"
failure_time=$($cli_without_timeout logs $pod_name | grep 'CSPFK010E Failed to authenticate' | head -1 | awk '{ print $3 }' | awk -F: '{ print ($1 * 3600) + ($2 * 60) + int($3) }')

echo "Expecting Secrets Provider retry configurations to take defaults RETRY_INTERVAL_SEC $DEFAULT_RETRY_INTERVAL_SEC and RETRY_COUNT_LIMIT $DEFAULT_RETRY_COUNT_LIMIT"
$cli_with_timeout "logs $pod_name | grep 'CSPFK010I Updating Kubernetes Secrets: 1 retries out of $DEFAULT_RETRY_COUNT_LIMIT'"
retry_time=$($cli_without_timeout logs $pod_name | grep "CSPFK010I Updating Kubernetes Secrets: 1 retries out of $DEFAULT_RETRY_COUNT_LIMIT" | head -1 | awk '{ print $3 }' | awk -F: '{ print ($1 * 3600) + ($2 * 60) + int($3) }')

duration=$(( retry_time - failure_time ))

# Since we are testing retry in scripts we must determine an acceptable range that retry should have taken place
# If the duration falls within that range, then we can determine the retry mechanism works as expected
retryIntervalMin=`echo "scale=3; $DEFAULT_RETRY_INTERVAL_SEC/100*80" | bc -l | awk '{print ($0-int($0)<0.499)?int($0):int($0)+1}'`
retryIntervalMax=`echo "scale=3; $DEFAULT_RETRY_INTERVAL_SEC/100*160" | bc -l | awk '{print ($0-int($0)<0.499)?int($0):int($0)+1}'`
if (( ($duration >= $retryIntervalMin) && ($duration <= $retryIntervalMax) || $duration == 0 )); then
  exit 0
else
  echo "Timed retry failed to occur according to detailed retry interval. Timed duration: $duration"
  exit 1
fi

