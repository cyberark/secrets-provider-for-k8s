#!/bin/bash

set -euo pipefail
cat << EOL
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${TEST_APP_NAMESPACE_NAME}-sa
---
apiVersion: v1
kind: RoleBinding
metadata:
  namespace: ${TEST_APP_NAMESPACE_NAME}
  name: secrets-access-role-binding
subjects:
  - kind: ServiceAccount
    name: ${TEST_APP_NAMESPACE_NAME}-sa
    namespace: ${TEST_APP_NAMESPACE_NAME}
roleRef:
  kind: ClusterRole
  name: secrets-access
EOL
