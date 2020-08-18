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

  DEFAULT_RETRY_INTERVAL_SEC=30
  DEFAULT_RETRY_COUNT_LIMIT=3

  fill_helm_chart_test_image
  fill_helm_chart_no_override_defaults
  helm install -f "../helm/secrets-provider/ci/take-default-test-values.yaml" \
    -f "../helm/secrets-provider/ci/take-image-values.yaml" \
    secrets-provider ../helm/secrets-provider \
    --set-file environment.conjur.sslCertificate.value="test/test_cases/conjur.pem"
popd

pod_name=$($cli_with_timeout get pods --namespace=$APP_NAMESPACE_NAME --selector app=test-helm --no-headers | awk '{print $1}' )

# Find initial authentication error that should trigger the retry
$cli_with_timeout "logs $pod_name | grep 'CSPFK010E Failed to authenticate'"
# Start the timer for retry interval
start=$SECONDS

echo "Expecting Secrets Provider retry configurations to take defaults RETRY_INTERVAL_SEC 30 and RETRY_COUNT_LIMIT 3"
$cli_with_timeout "logs $pod_name | grep 'CSPFK010I Updating Kubernetes Secrets: 1 retries out of $DEFAULT_RETRY_COUNT_LIMIT'"

duration=$(( SECONDS - start ))
# Since we are testing retry in scripts we must determine an acceptable range that retry should have taken place
# If the duration falls within that range, then we can determine the retry mechanism works as expected
retryIntervalMin=`echo "scale=2; $DEFAULT_RETRY_INTERVAL_SEC/100*80" | bc`
retryIntervalMax=`echo "scale=2; $DEFAULT_RETRY_INTERVAL_SEC/100*120" | bc`
if (( $duration >= $retryIntervalMin && $duration <= $retryIntervalMax )); then
  echo 0
else
  echo 1
fi

