#!/bin/sh

# This script checks for the existence of a 'CONJUR_SECRETS_UPDATED' sentinel
# file. This sentinel file gets created by the Secrets Provider whenever
# secret files or Kubernetes Secrets have been updated. When the file is
# detected, it is deleted, and the script returns an exit status of '1'.
#
# If an application container:
#
# - Is not already making use of a Kubernetes 'livenessProbe', AND...
# - Contains a bourne shell binary ('/bin/sh')
#
# then this script can be used in a livenessProbe definition to implement
# a file watcher for 'CONJUR_SECRETS_UPDATED'. When used this way, the
# application container will be restarted by Kubernetes whenever the
# presence of the 'CONJUR_SECRETS_UPDATED' file is detected.

cd "$(dirname "$0")"
if [ -f ./CONJUR_SECRETS_UPDATED ]; then
    rm ./CONJUR_SECRETS_UPDATED
    exit 1
fi
