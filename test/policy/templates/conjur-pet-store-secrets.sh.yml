#!/bin/bash

set -euo pipefail
cat << EOL
---
- !policy
  id: secrets
  body:
  - &variables
    - !variable db_username
    - !variable db_password

- !permit
  resources: *variables
  role: !host conjur/authn-k8s/${AUTHENTICATOR_ID}/apps/${TEST_APP_NAMESPACE_NAME}/*/*
  privileges: [ read, execute ]
EOL
