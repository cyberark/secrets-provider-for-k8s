#!/bin/bash

main() {
  announce "Validating that the deployments are functioning as expected."

  init_api_pod=$($cli get pods --no-headers -l app=pet-store-env | awk '{ print $1 }')

  if [[ "$init_api_pod" != "" ]]; then
      echo "Init Container + REST API: $($cli exec -c $TEST_APP_NAMESPACE_NAME-app $init_api_pod -- /webapp_v5.sh)"
  fi
}
