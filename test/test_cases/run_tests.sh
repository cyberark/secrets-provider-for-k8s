#!/bin/bash
set -euo pipefail

# By default lookup for folders with specifics prefix of type 'test_'. Can be modified by passing argument.
TEST_NAME_PREFIX=${1:-TEST_ID_}

# Keep environment variables for debugging
printenv > printenv.debug

export TEST_CASES_K8S_CONFIG_DIR="$PWD/../k8s-config"

# export all utils.sh functions to be available for all tests
set -a
source "../utils.sh"
set +a

./test_case_teardown.sh

times=1

for c in {1..$times}
do
  for filename in ./$TEST_NAME_PREFIX*.sh; do
      announce "Running '$filename'."
      ./test_case_setup.sh
      $filename
      ./test_case_teardown.sh
      announce "Test '$filename' ended successfully"
  done
done

rm printenv.debug
