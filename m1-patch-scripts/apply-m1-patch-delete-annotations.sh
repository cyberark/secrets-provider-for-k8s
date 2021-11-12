#!/bin/bash

set -e

source ./utils.sh

apply_deployment_patch app-test test-app-secrets-provider-init m1-delete-annotations.yaml
#kubectl patch deployment -n app-test test-app-secrets-provider-init --patch "$(cat m1-delete-annotations.yaml)"
