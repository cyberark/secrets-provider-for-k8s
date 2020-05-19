#!/bin/bash

# Use credentials provided by secrets-provider-for-k8s to connect to DB,
# add entries, and fetch entries
init_url=$($cli describe service pet-store-env|
    grep 'LoadBalancer Ingress' | awk '{ print $3 }'):8080

main() {
    echo -e "Adding entry to the init app\n"
    curl \
      -d '{"name": "Mr. Init"}' \
      -H "Content-Type: application/json" \
      $init_url/pet

    echo -e "Querying init app\n"
    curl $init_url/pets
}