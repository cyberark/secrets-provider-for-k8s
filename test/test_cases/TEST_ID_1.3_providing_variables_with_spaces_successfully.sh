#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

test_secret_is_provided "var_with_spaces_secret" "secrets/var with spaces" "VARIABLE_WITH_SPACES_SECRET"
