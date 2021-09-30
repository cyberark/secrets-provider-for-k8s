#!/bin/bash

kubectl patch deployment -n app-test test-app-secrets-provider-init --patch "$(cat patch-modify-config-file.yaml)"

