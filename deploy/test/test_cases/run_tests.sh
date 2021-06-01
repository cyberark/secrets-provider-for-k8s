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

for c in {1..$times}
do
  for filename in ./$TEST_NAME_PREFIX*.sh; do
    announce "Running '$filename'."
    ./test_case_setup.sh
    $filename
    ../../teardown_resources.sh
    announce "Test '$filename' ended successfully"
  done
done

ENV_FILE=printenv.debug
if [[ -f "$ENV_FILE" ]]; then
    rm $ENV_FILE
fi
