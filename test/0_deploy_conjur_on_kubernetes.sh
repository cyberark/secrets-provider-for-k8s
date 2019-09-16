#!/bin/bash

pushd .
  git clone --single-branch --branch master git@github.com:cyberark/kubernetes-conjur-deploy kubernetes-conjur-deploy-$UNIQUE_TEST_ID
popd || return

# Deploy k8s conjur environment
pushd .
  cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID || return
  ./start
popd || return
