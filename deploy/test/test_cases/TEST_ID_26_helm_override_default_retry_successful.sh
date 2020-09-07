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
  helm install -f "../helm/secrets-provider/ci/test-values.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur.pem"
popd

$cli_with_timeout "get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers"
pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers | awk '{print $1}' )
# Find initial authentication error that should trigger the retry
$cli_with_timeout "logs $pod_name | grep 'CSPFK010E Failed to authenticate'"
# Start the timer for retry interval
start=$SECONDS

# Validate that the Secrets Provider retry mechanism takes user input of RETRY_INTERVAL_SEC of 5 and RETRY_COUNT_LIMIT of 2
echo "Expecting Secrets Provider retry configurations to take their defaults of RETRY_INTERVAL_SEC of 5 and RETRY_COUNT_LIMIT of 2"
$cli_with_timeout "logs $pod_name | grep 'CSPFK010I Updating Kubernetes Secrets: 1 retries out of $RETRY_COUNT_LIMIT'"

duration=$(( SECONDS - start ))
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

