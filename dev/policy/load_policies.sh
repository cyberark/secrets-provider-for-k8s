#!/bin/bash
set -eo pipefail

if [ "$CONJUR_APPLIANCE_URL" != "" ]; then
  echo "Running conjur init with $CONJUR_APPLIANCE_URL"
  conjur init -u $CONJUR_APPLIANCE_URL -a $CONJUR_ACCOUNT
fi

# check for unset vars after checking for appliance url
set -u

echo "Login to Conjur with the conjur-cli"
conjur authn login -u admin -p $CONJUR_ADMIN_PASSWORD

readonly POLICY_DIR="/policy"

# NOTE: generated files are prefixed with the test app namespace to allow for parallel CI
readonly POLICY_FILES=(
  "$POLICY_DIR/users.yml"
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.project-authn.yml"
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.cluster-authn-svc.yml"
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-secrets.yml"
)

for policy_file in "${POLICY_FILES[@]}"; do
  echo "Loading policy $policy_file..."
  conjur policy load root $policy_file
done

conjur variable values add secrets/test_secret "supersecret"

conjur authn logout
