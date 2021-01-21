#!/bin/bash
set -euxo pipefail

# This test verifies that user configured values for retry mechanism work
setup_helm_environment

pushd ../../
  export LABELS="app: test-helm"
  export DEBUG="true"
  # A parameter that will force a failure
  export CONJUR_AUTHN_URL="https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local/api/authn-k8s/${AUTHENTICATOR_ID}xyz"  # Configure retry mechanism with overriding defaults
  export RETRY_COUNT_LIMIT="2"
  export RETRY_INTERVAL_SEC="5"
  fill_helm_chart
  helm install -f "../helm/secrets-provider/ci/test-values-$UNIQUE_TEST_ID.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur-$UNIQUE_TEST_ID.pem"
popd

pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-helm')"

# Find initial authentication error that should trigger the retry
$cli_with_timeout "logs $pod_name | grep 'CSPFK010E Failed to authenticate'" | head -1
failure_time=$($cli_without_timeout logs $pod_name | grep 'CSPFK010E Failed to authenticate' | head -1 | awk '{ print $3 }' | awk -F: '{ print ($1 * 3600) + ($2 * 60) + int($3) }')


# Validate that the Secrets Provider retry mechanism takes user input of RETRY_INTERVAL_SEC of 5 and RETRY_COUNT_LIMIT of 2
echo "Expecting Secrets Provider retry configurations to take their defaults of RETRY_INTERVAL_SEC of 5 and RETRY_COUNT_LIMIT of 2"
$cli_with_timeout "logs $pod_name | grep 'CSPFK010I Updating Kubernetes Secrets: 1 retries out of $RETRY_COUNT_LIMIT'" | head -1
retry_time=$($cli_without_timeout logs $pod_name | grep "CSPFK010I Updating Kubernetes Secrets: 1 retries out of $RETRY_COUNT_LIMIT" | head -1 | awk '{ print $3 }' | awk -F: '{ print ($1 * 3600) + ($2 * 60) + int($3) }')

duration=$(( retry_time - failure_time ))
# Since we are testing retry in scripts we must determine an acceptable range that retry should take place
# If the duration falls within that range, then we can determine the retry mechanism works as expected
retryIntervalMin=`echo "scale=2; $RETRY_INTERVAL_SEC/100*80" | bc -l | awk '{print ($0-int($0)<0.499)?int($0):int($0)+1}'`
retryIntervalMax=`echo "scale=2; $RETRY_INTERVAL_SEC/100*160" | bc -l | awk '{print ($0-int($0)<0.499)?int($0):int($0)+1}'`
if (( $duration >= $retryIntervalMin && $duration <= $retryIntervalMax )); then
  exit 0
else
  echo "Timed retry failed to occur according to detailed retry interval. Timed duration: $duration"
  exit 1
fi

