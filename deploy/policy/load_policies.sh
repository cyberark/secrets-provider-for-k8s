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

readonly POLICY_DIR="/tmp/policy"

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
conjur variable set -i secrets/another_test_secret -v "some-secret"
conjur variable set -i "secrets/ssh_key" -v "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\""
conjur variable set -i "secrets/json_object_secret" -v "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\""
conjur variable set -i "secrets/var with spaces" -v "some-secret"
conjur variable set -i "secrets/var+with+pluses" -v "some-secret"
conjur variable set -i "secrets/umlaut" -v "ÄäÖöÜü"
conjur variable set -i "secrets/encoded" -v "$(echo "secret-value" | tr -d '\n' | base64)" # == "c2VjcmV0LXZhbHVl"
conjur variable set -i secrets/url -v "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
conjur variable set -i secrets/username -v "some-user"
conjur variable set -i secrets/password -v "7H1SiSmYp@5Sw0rd"

conjur logout
