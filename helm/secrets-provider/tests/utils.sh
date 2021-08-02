#!/bin/bash

colorize="${COLORIZE:-true}"

readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly BLUE='\033[0;34m'
readonly NOCOLOR='\033[0m'
readonly BOLD='\e[1m'
readonly ANNOUNCE_COLOR="$BLUE"
readonly MIN_HELM_VERSION="3.5.3"
readonly EXPECT_FAILURE=true

# Set the current text color if colorizing is enabled.
function set_color() {
    if [ "$colorize" = true ]; then
        echo -e "$1"
    else
        echo -e ""
    fi
}

# If colorizing is enabled, temporarily set the text color, print a string,
# and then reset the text color, without any intervening newlines. This can
# be useful for printing just a word or a portion of a line in color. If
# colorizing is disabled, it simply prints the string.
function color_text() {
    if [ "$colorize" = true ]; then
        echo -en "$1"
    fi
    echo "${@:2}"
    if "$colorize"; then
        echo -en "$NOCOLOR"
    fi
}

# Print a string inside leading and trailing lines of dashes. If colorizing
# is enabled, print everything in the ANNOUNCE_COLOR.
function announce() {
  set_color "$ANNOUNCE_COLOR"
  echo "---------------------------------------------------------------------"
  echo -e "$@"
  echo "---------------------------------------------------------------------"
  set_color "$NOCOLOR"
}

# Print a string inside leading and trailing lines of '=' characters. If
# colorizing is enabled, print everything in a given color.
function banner() {
  set_color "$1"
  echo "====================================================================="
  echo -e "${@:2}"
  echo "====================================================================="
  set_color "$NOCOLOR"
}

# Compare two semantic versions, and return the oldest of the two.
function oldest_version() {
  v1=$1
  v2=$2

  echo "$(printf '%s\n' "$v1" "$v2" | sort -V | head -n1)"
}

# Return true if a given semantic version is the same as or newer than
# a given minimum acceptable version. Return false otherwise.
function meets_min_version() {
  actual_version=$1
  min_version=$2

  oldest="$(oldest_version $actual_version $min_version)"
  if [ "$oldest" = "$min_version" ]; then
    true
  else
    false
  fi
}

function invert_exit_status() {
    exit_status="$1"
    if [ "$exit_status" -ne 0 ]; then
        echo 0
    else
        echo 1
    fi
}

function update_results() {
    exit_status="$1"
    failure_expected="${2:-false}"
    if [ "$failure_expected" = true ]; then
        echo "FAILURE EXPECTED"
        exit_status="$(invert_exit_status $exit_status)"
    fi
    if [ "$exit_status" -eq 0 ]; then
        color_text "$GREEN" "Test Case PASSED!"
        let "num_passed=num_passed+1"
    else
        color_text "$RED" "Test Case FAILED!"
        let "num_failed=num_failed+1"
        test_failed=true
    fi
}

function display_final_results() {
    if [ "$num_failed" -eq 0 ]; then
        result_text="Test PASSED!"
        result_color="$GREEN"
    else
        result_text="Test FAILED!"
        result_color="$RED"
    fi
    banner "$result_color" \
        "$result_text\n" \
        "   Passed test cases: $num_passed" \
        "   Failed test cases: $num_failed"
}

function check_helm_version() {
    helm_version="$(helm version --template {{.Version}} | sed 's/^v//')"
    if ! meets_min_version $helm_version $MIN_HELM_VERSION; then
        echo "helm version $helm_version is invalid. Version must be $MIN_HELM_VERSION or newer"
        exit 1
    fi
}
