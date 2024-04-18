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

# Uncomment for Golang-based tests
./test_case_setup.sh
create_secret_access_role
create_secret_access_role_binding
deploy_env
pushd /secrets-provider-for-k8s
go test -v -tags e2e -timeout 0 ./e2e/...
popd

../../teardown_resources.sh
