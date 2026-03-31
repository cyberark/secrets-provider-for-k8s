#!/bin/bash
set -euo pipefail

_RUN_TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_DEPLOY_DIR="$(cd "${_RUN_TESTS_DIR}/../.." && pwd)"
_REPO_ROOT="$(cd "${_DEPLOY_DIR}/.." && pwd)"
_TEARDOWN_SCRIPT="${_DEPLOY_DIR}/teardown_resources.sh"

# By default lookup for folders with specifics prefix of type 'test_'.
# Can be modified by using the `--test-prefix` flag for running `./bin/start`.
TEST_NAME_PREFIX=${TEST_NAME_PREFIX:-TEST_ID_}

# Keep environment variables for debugging
printenv > printenv.debug

export CONFIG_DIR="${_DEPLOY_DIR}/config/k8s"
if [[ "$PLATFORM" = "openshift" ]]; then
    export CONFIG_DIR="${_DEPLOY_DIR}/config/openshift"
fi

# export all utils.sh functions to be available for all tests
set -a
source "${_DEPLOY_DIR}/utils.sh"
set +a

build_e2e_failure_report() {
  local test_log_file="$1"
  local summary_file="$2"
  local signatures_file="$3"

  local first_failure_line=""
  local failure_header=""
  local likely_cause="Unknown"
  local failure_lines_tmp

  failure_lines_tmp="$(mktemp)"

  if grep -n -- "--- FAIL:" "$test_log_file" > "$failure_lines_tmp" 2>/dev/null; then
    first_failure_line="$(head -n 1 "$failure_lines_tmp" | cut -d: -f1)"
    failure_header="$(head -n 1 "$failure_lines_tmp" | cut -d: -f2-)"
  fi

  if grep -Eq "unable to upgrade connection: container not found" "$test_log_file"; then
    likely_cause="Pod/container readiness race during kubectl exec"
  elif grep -Eq "timed out waiting|context deadline exceeded|i/o timeout|connection refused" "$test_log_file"; then
    likely_cause="Cluster or service readiness/connectivity timeout"
  elif grep -Eq "secret .* not found|couldn't find key" "$test_log_file"; then
    likely_cause="Expected Kubernetes secret key missing at assertion time"
  elif grep -Eq "Error from server \(NotFound\)|No resources found" "$test_log_file"; then
    likely_cause="Resources missing during validation or diagnostics window"
  fi

  {
    echo "E2E Failure Analysis Summary"
    echo "Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Log file: ${test_log_file}"
    if [[ -n "$first_failure_line" ]]; then
      echo "First failing test line: ${first_failure_line}"
      echo "Failure header: ${failure_header}"
    else
      echo "First failing test line: not found"
      echo "Failure header: none"
    fi
    echo "Likely cause: ${likely_cause}"
    echo ""
    echo "Top failure signatures:"
    local sig_count
    if ! sig_count=$(grep -cE -- "--- FAIL:|unable to upgrade connection|container not found|timed out waiting|context deadline exceeded|i/o timeout|connection refused|couldn't find key|secret .* not found|Error from server \(NotFound\)|No resources found" "$test_log_file" 2>/dev/null); then
      sig_count=0
    fi
    grep -nE -- "--- FAIL:|unable to upgrade connection|container not found|timed out waiting|context deadline exceeded|i/o timeout|connection refused|couldn't find key|secret .* not found|Error from server \(NotFound\)|No resources found" "$test_log_file" | head -n 80 || true
    if [[ "$sig_count" -gt 80 ]]; then
      echo "...truncated (${sig_count} total matches, showing first 80)"
    fi
  } > "$summary_file"

  grep -nE "unable to upgrade connection|container not found|timed out waiting|context deadline exceeded|i/o timeout|connection refused|couldn't find key|secret .* not found|Error from server \(NotFound\)|No resources found" "$test_log_file" > "$signatures_file" || true

  rm -f "$failure_lines_tmp"
}

"${_TEARDOWN_SCRIPT}"

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

"${_RUN_TESTS_DIR}/test_case_setup.sh"
create_secret_access_role
create_secret_access_role_binding
deploy_env
test_output_dir="${_REPO_ROOT}/deploy/output"
export TEST_OUTPUT_DIR="$test_output_dir"

pushd "${_REPO_ROOT}"
mkdir -p "$test_output_dir"
test_log_file="$test_output_dir/e2e-go-test.log"
summary_file="$test_output_dir/e2e-analysis-summary.txt"
signatures_file="$test_output_dir/e2e-failure-signatures.txt"

announce "Building Helm chart dependencies"
if ! command -v helm >/dev/null 2>&1; then
  echo "ERROR: helm not found in PATH; cannot build chart dependencies" >&2
  exit 1
fi
helm repo add stakater https://stakater.github.io/stakater-charts 2>/dev/null || true
helm repo update
helm dependency build "${_REPO_ROOT}/helm/secrets-provider"

if [[ -n "$testNameRegex" ]]; then
  echo "Running tests matching regex: $testNameRegex"
  test_cmd=(go test -v -tags e2e -run="$testNameRegex" -timeout 0 ./e2e/...)
else
  test_cmd=(go test -v -tags e2e -timeout 0 ./e2e/...)
fi

set +e
"${test_cmd[@]}" 2>&1 | tee "$test_log_file"
test_exit_code=${PIPESTATUS[0]}
set -e

if [[ "$test_exit_code" -ne 0 ]]; then
  build_e2e_failure_report "$test_log_file" "$summary_file" "$signatures_file"
  echo "E2E test failure report generated: $summary_file"
  echo "E2E failure signatures generated: $signatures_file"
else
  {
    echo "E2E Failure Analysis Summary"
    echo "Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Log file: ${test_log_file}"
    echo "Result: success"
  } > "$summary_file"
fi

popd

if [[ "$test_exit_code" -ne 0 ]]; then
  exit "$test_exit_code"
fi

"${_TEARDOWN_SCRIPT}"
