#!/bin/bash

resource="$(kubectl get pod -n app-test -l app=test-app-secrets-provider-init -o name)"
pod_name="${resource##*/}"
echo "Showing contents of secrets files in Pod " "$pod_name"
kubectl exec "$pod_name" -c test-app -- /bin/sh -c 'for i in `ls /conjur/secrets/*`; do echo; echo $i; echo ----------------------; cat $i; echo; echo ======================; echo; done'
