#!/bin/sh
set -eo pipefail

if [ "$CONJUR_APPLIANCE_URL" != "" ]; then
  echo "Running conjur init with $CONJUR_APPLIANCE_URL"
  conjur init -u $CONJUR_APPLIANCE_URL -a $CONJUR_ACCOUNT --self-signed --force
fi

# check for unset vars after checking for appliance url
set -u

echo "Login to Conjur with the conjur-cli"
conjur login -i admin -p $CONJUR_ADMIN_PASSWORD

readonly POLICY_DIR="/policy"

# NOTE: generated files are prefixed with the test app namespace to allow for parallel CI
set -- "$POLICY_DIR/users.yml" \
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.project-authn.yml" \
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.cluster-authn-svc.yml" \
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.app-identity.yml" \
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-secrets.yml" \
  "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.authn-any-policy-branch.yml"

for policy_file in "$@"; do
  echo "Loading policy $policy_file..."
  conjur policy load -b root -f "$policy_file"
done

# the values of these secrets aren't important as we populate the secret that we
# are testing in each test. We need them to have some value as both are required
# in the pod
conjur variable set -i secrets/test_secret -v "some-secret"
conjur variable set -i "secrets/var with spaces" -v "some-secret"
conjur variable set -i "secrets/var+with+pluses" -v "some-secret"
conjur variable set -i "secrets/umlaut" -v "ÄäÖöÜü"
conjur variable set -i "secrets/encoded" -v "$(echo "secret-value" | tr -d '\n' | base64)" # == "c2VjcmV0LXZhbHVl"
conjur variable set -i secrets/url -v "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
conjur variable set -i secrets/username -v "some-user"
conjur variable set -i secrets/password -v "7H1SiSmYp@5Sw0rd"

conjur logout
