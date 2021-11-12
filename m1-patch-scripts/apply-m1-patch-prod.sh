#!/bin/bash

set -e

source ./utils.sh

apply_deployment_patch app-test test-app-secrets-provider-init m1-patch-prod.yaml
