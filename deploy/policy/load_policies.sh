#!/bin/sh
set -e

url_encode() {
  # URL encode a string - handles spaces, slashes, and other special characters
  echo "$1" | sed 's| |%20|g; s|/|%2F|g; s|+|%2B|g; s|:|%3A|g; s|@|%40|g; s|#|%23|g; s|\$|%24|g; s|&|%26|g; s|"|%22|g; s|'\''|%27|g; s|\[|%5B|g; s|\]|%5D|g'
}

load_policy() {
  branch=$1
  policy_file=$2

  if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
    curl -w "%{http_code}" -H "Authorization: Token token=\"$INFRAPOOL_CONJUR_AUTHN_TOKEN\"" \
      -X POST -d "$(cat $policy_file)" "${CONJUR_APPLIANCE_URL}/policies/conjur/policy/$branch"
  else 
    conjur policy load -b "$branch" -f "/$policy_file"
  fi
}

set_variable() {
  variable_name=$1
  variable_value=$2

  if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
    curl -w "%{http_code}" -H "Authorization: Token token=\"$INFRAPOOL_CONJUR_AUTHN_TOKEN\"" \
      -X POST --data "${variable_value}" "${CONJUR_APPLIANCE_URL}/secrets/conjur/variable/$(url_encode "$variable_name")"
  else 
    conjur variable set -i "$variable_name" -v "$variable_value"
  fi
}

enable_authenticator() {
  service_id=$1
  if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
    curl -w "%{http_code}" -H "Authorization: Token token=\"$INFRAPOOL_CONJUR_AUTHN_TOKEN\"" \
        -X PATCH  -H 'Content-Type: application/json' -H "Accept: application/x.secretsmgr.v2beta+json" --data '{"enabled":true}' "${CONJUR_APPLIANCE_URL}/authenticators/jwt/$service_id"

    # check status endpoint to verify that the authenticator is enabled
    curl -w "%{http_code}" -H "Authorization: Token token=\"$INFRAPOOL_CONJUR_AUTHN_TOKEN\"" \
      -X GET "${CONJUR_APPLIANCE_URL}/authn-jwt/$service_id/conjur/status"
  fi
}

if [ "$CONJUR_DEPLOYMENT" != "cloud" ]; then
  if [ "$CONJUR_APPLIANCE_URL" != ""  ]; then
    echo "Running conjur init with $CONJUR_APPLIANCE_URL"
    conjur init -u $CONJUR_APPLIANCE_URL -a $CONJUR_ACCOUNT --self-signed --force
  fi

  # check for unset vars after checking for appliance url
  set -u

  echo "Login to Conjur with the conjur-cli"
  conjur login -i admin -p $CONJUR_ADMIN_PASSWORD
fi

# For cloud deployments, use the local policy directory; otherwise use /tmp/policy (in pod)
if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
  # Use the directory containing this script as POLICY_DIR
  readonly POLICY_DIR="$(cd "$(dirname "$0")" && pwd)"
else
  readonly POLICY_DIR="/tmp/policy"
fi

# NOTE: generated files are prefixed with the test app namespace to allow for parallel CI
set -x
if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
  echo
  cat $POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-host.yml
  echo
  load_policy data "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-host.yml"
  echo
  cat $POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-secrets.yml
  echo
  load_policy data "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-secrets.yml"
  echo
  cat $POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-authn.yml
  echo
  load_policy conjur/authn-jwt "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-authn.yml"
  echo
  echo
else
  load_policy root "$POLICY_DIR/users.yml"
  load_policy root "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-authn.yml"
  load_policy root "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.cluster-authn-svc.yml"
  load_policy root "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.app-identity.yml"
  load_policy root "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-secrets.yml"
  load_policy root "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.authn-any-policy-branch.yml"
fi

# the values of these secrets aren't important as we populate the secret that we
# are testing in each test. We need them to have some value as both are required
# in the pod
set_variable data/secrets/test_secret "some-secret"
set_variable data/secrets/another_test_secret "some-secret"
set_variable data/secrets/ssh_key "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\""
set_variable "data/secrets/json_object_secret" "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\""
set_variable "data/secrets/var with spaces" "some-secret"
set_variable "data/secrets/var+with+pluses" "some-secret"
set_variable "data/secrets/umlaut" "ÄäÖöÜü"
set_variable "data/secrets/encoded" "$(echo "secret-value" | tr -d '\n' | base64)" # == "c2VjcmV0LXZhbHVl"
set_variable data/secrets/url "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
set_variable data/secrets/username "some-user"
set_variable data/secrets/password "7H1SiSmYp@5Sw0rd"

echo "Adding authn-jwt variables"
set_variable "conjur/authn-jwt/$AUTHENTICATOR_ID/issuer" "$ISSUER"
if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
  set_variable "conjur/authn-jwt/$AUTHENTICATOR_ID/identity-path" "data"
  set_variable "conjur/authn-jwt/$AUTHENTICATOR_ID/token-app-property" "sub"
  set_variable "conjur/authn-jwt/$AUTHENTICATOR_ID/public-keys" "$PUBLIC_KEYS"
  enable_authenticator "$AUTHENTICATOR_ID"
else
  set_variable "conjur/authn-jwt/$AUTHENTICATOR_ID/identity-path" "conjur/authn-jwt/$AUTHENTICATOR_ID/apps"
  set_variable "conjur/authn-jwt/$AUTHENTICATOR_ID/jwks-uri" "$JWKS_URI"
  set_variable "conjur/authn-jwt/$AUTHENTICATOR_ID/ca-cert" "$(echo $CA_CERT_B64 | base64 -d)"
fi
set +x

if [ "$CONJUR_DEPLOYMENT" != "cloud" ]; then
  echo "Adding authn-azure variables"
  set_variable "conjur/authn-azure/$AUTHENTICATOR_ID/provider-uri" "$PROVIDER_URI"

  conjur logout
fi
