#!/bin/sh

# This is a convenient script that can be used in a Kubernetes 'postStart'
# lifecycle hook for the Secrets Provider container in an application
# Deployment manifest. When used in this manner, Kubernetes will hold off on
# starting other containers until after Secrets Provider has provided
# Conjur secrets via secret files or Kubernetes Secrets.

until [ -f /conjur/status/CONJUR_SECRETS_PROVIDED ]; do
    sleep 1
done
