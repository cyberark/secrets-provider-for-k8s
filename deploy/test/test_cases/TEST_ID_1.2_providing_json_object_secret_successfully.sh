#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

test_secret_is_provided "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\""
