#!/bin/bash
set -euo pipefail

# By default lookup for folders with specifics prefix of type 'test_'.
# Can be modified by using the `--test-prefix` flag for running `./bin/start`.
TEST_NAME_PREFIX=${TEST_NAME_PREFIX:-TEST_ID_}

# Keep environment variables for debugging
printenv > printenv.debug

export CONFIG_DIR="$PWD/../../config/k8s"
if [[ "$PLATFORM" = "openshift" ]]; then
    export CONFIG_DIR="$PWD/../../config/openshift"
fi

# export all utils.sh functions to be available for all tests
set -a
source "../../utils.sh"
set +a

../../teardown_resources.sh

times=1

announce "Preparing to run E2E tests"

# Ensure Go and Kind binaries are in PATH (installed in Dockerfile.e2e)
export PATH="${PATH}:/usr/local/go/bin:/root/go/bin"

# If running KinD based cloud authn tests, only run those test cases.
# We don't want to rerun all the other tests in KinD as they are run in other environments.
testNameRegex=""
if [[ "${E2E_AUTHN_AZURE:-false}" = "true" ]]; then
  testNameRegex="TestAuthnAzure"
  echo "Running only Authn Azure tests in KinD environment."
elif [[ "${E2E_AUTHN_IAM:-false}" = "true" ]]; then
  testNameRegex="TestAuthnIAM"
  echo "Running only Authn IAM tests in KinD environment."
fi

./test_case_setup.sh
create_secret_access_role
create_secret_access_role_binding
deploy_env
pushd /secrets-provider-for-k8s
if [[ -n "$testNameRegex" ]]; then
  echo "Running tests matching regex: $testNameRegex"
  go test -v -tags e2e -run="$testNameRegex" -timeout 0 ./e2e/...
else
  go test -v -tags e2e -timeout 0 ./e2e/...
fi
popd

../../teardown_resources.sh
