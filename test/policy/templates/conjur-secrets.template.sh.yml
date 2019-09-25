#!/bin/bash

set -euo pipefail
cat << EOL
# Should be loaded into root
- !policy
  id: secrets
  body:
  - &variables
    - !variable test_secret

- !permit
  resources: *variables
  role: !host conjur/authn-k8s/${AUTHENTICATOR_ID}/apps/${TEST_APP_NAMESPACE_NAME}/*/*
  privileges: [ read, execute ]
EOL
