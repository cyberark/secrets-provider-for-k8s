#!/bin/bash
set -e

wait_for_it() {
  local timeout=$1
  local spacer=10
  shift

  if ! [ $timeout = '-1' ]; then
    local times_to_run=$((timeout / spacer))

    for i in $(seq $times_to_run); do
      printf "\rWait for %s, try %s of %s" "$@" "$i" "$times_to_run"
      eval $*  && echo "" && return 0
      sleep $spacer
    done

    echo ""

    # Last run evaluated. If this fails we return an error exit code to caller
    eval $*
  fi
}

get_logs() {
  for (( i = 1; i <= secrets_providers_count; i++ ))
  do
    id=$1-$2-$i
    printf "\rGet logs from %s" $id
#      echo -e "$secrets_providers_count\t$repeat_num\t$i" >> log-50-5.log
#      $cli logs -f pods/test-env-$id >> log-50-5.log
    $cli logs -f pods/test-env-$id | grep -e 'QQQ' >> log-$1-$3.log
  done
  echo ""
}

cli="oc"

cli_with_timeout="wait_for_it 6000"
repo_root_path=$(git rev-parse --show-toplevel)

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

for secrets_providers_count in 1 #00 150 200 250
do
  for repeat_num in {1..1}
  do
    start=$(date +%s)
    test_duration_minutes=1
    test_duration_seconds=$(( test_duration_minutes*60 ))
    end_at=$(( start + test_duration_seconds ))
    printf "Run %d secrets providers for %d minutes repeat %d\n" $secrets_providers_count $test_duration_minutes $repeat_num

    for (( i = 1; i <= secrets_providers_count; i++ ))
    do
      id=$secrets_providers_count-$repeat_num-$i
      sed -e "s#{{ ID }}#$id#g" "$repo_root_path/deploy/test-env-1.yml" |
      sed -e "s#{{ END_AT }}#$end_at#g" |
        $cli create -f - > /dev/null
      printf "\rCreated %s" "$id"
    done
    echo ""

    echo "It took $(( $(date +%s) - start )) seconds to run $secrets_providers_count secrets providers"

    while (( $(date +%s) < end_at ))
    do
      printf "\r%d seconds left for test on %d secrets providers repeat %d   " $(( end_at - $(date +%s) )) $secrets_providers_count $repeat_num
      sleep 1
    done
    echo ""

    echo "Wait for secrets providers to finish execution"

    wait_for_it 6000 "oc get pods | grep -e Completed -e Error | wc -l | grep $secrets_providers_count > /dev/null"

    echo "Aggregate logs from pods"

    get_logs $secrets_providers_count $repeat_num $test_duration_minutes

    echo "Finished running $secrets_providers_count secrets providers repeat $repeat_num in $(( $(date +%s) - start )) seconds"

  done
done