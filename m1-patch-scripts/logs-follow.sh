#!/bin/bash

resource="$(kubectl get pod -n app-test -l app=test-app-secrets-provider-init -o name)"
pod_name="${resource##*/}"
kubectl logs -f -n app-test "$pod_name" -c test-app
