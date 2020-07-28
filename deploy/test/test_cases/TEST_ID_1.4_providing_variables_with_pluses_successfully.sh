#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

test_secret_is_provided "var_with_pluses_secret" "secrets/var+with+pluses" "VARIABLE_WITH_PLUSES_SECRET"
