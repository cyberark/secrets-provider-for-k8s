#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

# In this test we assume that the secret is encoded with base64.
test_secret_is_provided_and_decoded "secret-value" "secrets/encoded" "VARIABLE_WITH_ENCODED_SECRET"