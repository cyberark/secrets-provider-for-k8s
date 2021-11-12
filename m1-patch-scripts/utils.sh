#!/bin/bash

colorize="${COLORIZE:-true}"
secrets_dir="/conjur/secrets"

readonly BLACK='\033[0;30m'
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[0;33m'
readonly BLUE='\033[0;34m'
readonly MAGENTA='\033[0;35m'
readonly CYAN='\033[0;36m'
readonly WHITE='\033[0;37m'
readonly NOCOLOR='\033[0m'

readonly BOLD='\e[1m'
readonly ANNOUNCE_COLOR="$BLUE"
readonly CODE_BLOCK_COLOR="$GREEN"

# Set the current text color if colorizing is enabled.
function set_color() {
    if [ "$colorize" = true ]; then
        echo -n -e "$1"
    else
        echo -n -e ""
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

# Print a block of code between leading and trailing dashed lines
# in CODE_BLOCK_COLOR.
function code_block() {
  banner_with_color $CODE_BLOCK_COLOR "$1"
}

# Print a string between leading and trailing dashed lines in a specified
# color.
function banner_with_color() {
  set_color "$1"
  echo "---------------------------------------------------------------------"
  echo -e "${@:2}"
  echo "---------------------------------------------------------------------"
  set_color "$NOCOLOR"
}

# Print a string inside leading and trailing lines of '=' characters. If
# colorizing is enabled, print everything in a given color.
function announce() {
  set_color "$ANNOUNCE_COLOR"
  echo "====================================================================="
  echo -e "$1"
  echo "====================================================================="
  set_color "$NOCOLOR"
}

function apply_deployment_patch() {
  namespace="$1"
  deployment="$2"
  patch_file="$3"
  
  echo
  announce "Applying kubectl patch to $deployment deployment"
  echo "Patch file '$patch_file':"
  code_block "$(cat $patch_file)"

  echo -n "Patching deployment ..............."
  kubectl patch deployment -n "$namespace" "$deployment" --patch "$(cat $patch_file)"
  wait_for_deployment_rollout "$namespace" "$deployment"

  announce "Displaying secret files"
  display_secret_files "$namespace" "$deployment"
}

function wait_for_deployment_rollout() {
  namespace="$1"
  deployment="$2"
  timeout_secs=60
  
  # Wait for deployment rollout to complete
  attempts=0
  rollout_status_cmd="kubectl rollout status deployment/$deployment -n $namespace"
  until $rollout_status_cmd || [ $attempts -eq $timeout_secs ]; do
    echo -n "."
    attempts=$((attempts + 1))
    sleep 1
  done

  # Wait for previous application Pod to terminate 
  #attempts=0
  #echo -n "Waiting for previous Pod to terminate ..."
  #pod_status_cmd="kubectl get pod -n $namespace -l app=$deployment"
  #while output="$($pod_status_cmd | grep "Terminating")" && [ -n "$output" ] || [ $attempts -eq $timeout_secs ]; do
  #  echo -n "."
  #  attempts=$((attempts + 1))
  #  sleep 1
  #done
  #echo
  #echo
}

function display_secret_files() {
  namespace="$1"
  deployment="$2"
 
  #resource="$(kubectl get pod -n $namespace -l app=$deployment --field-selector status.phase=Running -o name)"
  #pod_name="${resource##*/}"
  output="$(kubectl get pod -n $namespace -l app=$deployment | grep Running)"
  pod_name=${output%% *}
  output="$(kubectl exec $pod_name -c test-app -- /bin/sh -c "ls $secrets_dir")"
  if [ -z "$output" ]; then
    echo "No secrets files!"
    return
  fi
  files=($output)
  for file in "${files[@]}"; do
    file="$secrets_dir""/""$file"
    echo "Secret file '$file':"
    output="$(kubectl exec $pod_name -c test-app -- /bin/sh -c "cat $file")"
    code_block "$output"
  done
}
