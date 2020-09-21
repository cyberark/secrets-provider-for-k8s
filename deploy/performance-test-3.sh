#!/bin/bash
set -e

wait_for_it() {
  local timeout=$1
  local spacer=1
  shift

  if ! [ $timeout = '-1' ]; then
    local times_to_run=$((timeout / spacer))

    for ii in $(seq $times_to_run); do
      printf "\rWait for %s, try %s of %s" "$@" "$ii" "$times_to_run"
      eval $*  && echo "" && return 0
      sleep $spacer
    done

    echo ""

    # Last run evaluated. If this fails we return an error exit code to caller
    eval $*
  fi
}

test_secrets() {
  echo "test" $1 "secrets"
  result=$(( $RANDOM % 2 == 0))
  echo "result" $result
}


while true ; do
  case "$1" in
    --nobuild ) skip_build=true ; shift ;;
     * ) if [ -z "$1" ]; then break; else echo "$1 is not a valid option"; exit 1; fi;;
  esac
done

if [[ ! $skip_build ]]; then

  #	Build Secrets Provider
  $repo_root_path/bin/build

  #	tag and push the secrets provider image to openshift internal registry
  docker tag "secrets-provider-for-k8s:dev" "docker-registry-default.openshift-311.itd.conjur.net/app-performance/secrets-provider" &&
    docker push "docker-registry-default.openshift-311.itd.conjur.net/app-performance/secrets-provider"

fi

max_limit=1000
for (( step = 100; step >= 1; step /= 10 )); do
  echo "Step: $step"
  for (( i = max_limit - step*9;i < max_limit; i += step )); do
    echo "Step: $step, i: $i"

    k8s_secrets=$(eval "echo secrets-{1..$i} | tr ' ' ','" )
    sed "s#{{ secrets }}#$k8s_secrets#" test-env-3.yml |
      sed "s#{{ ID }}#$i#" |
      oc apply -f -

    echo "Try $i secrets"
    sleep 1
    wait_for_it 100 "oc get pod test-env-$i | grep -e Completed -e Error > /dev/null"

#    if [ $i -gt 43 ]; then
    if oc get pod test-env-$i | grep -e Error; then
#    if echo $i; then
        echo "$i secrets failed"
        break
    fi
  done
  max_limit=$i
done
echo "$((i - 1)) secrets is the max number"