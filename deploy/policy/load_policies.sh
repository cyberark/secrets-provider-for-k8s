#!/bin/sh
set -eo pipefail

cli_exec() {
  if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
    policy_dir="$(cd "$(dirname "$0")" && pwd)"
    docker run --rm -it -v "$policy_dir:/tmp/policy:ro" \
      -e CONJUR_APPLIANCE_URL="$INFRAPOOL_CONJUR_APPLIANCE_URL" \
      -e CONJUR_AUTHN_TOKEN="$(echo "$INFRAPOOL_CONJUR_AUTHN_TOKEN" | base64 -d)" \
      -e CONJUR_AUTHN_LOGIN="$INFRAPOOL_CONJUR_AUTHN_LOGIN" \
      -e CONJUR_ACCOUNT \
      -e CONJUR_AUTHN_TYPE=authn \
      cyberark/conjur-cli:latest \
      "$@"
  else
    # Script is already running in the configured CLI pod, just execute the command
    conjur "$@"
  fi
}

load_policy() {
  branch=$1
  policy_file=$2
  cli_exec policy load -b "$branch" -f "$policy_file"
}

set_variable() {
  variable_name=$1
  variable_value=$2
  cli_exec variable set -i "$variable_name" -v "$variable_value"
}

enable_authenticator() {
  service_id=$1
  cli_exec authenticator enable -i "authn-jwt/$service_id"
}

main() {
  # Same path for both: cloud mounts host policy dir at /tmp/policy in cli_exec
  readonly POLICY_DIR="/tmp/policy"

  # NOTE: generated files are prefixed with the test app namespace to allow for parallel CI
  if [ "$CONJUR_DEPLOYMENT" = "cloud" ]; then
    load_policy data "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-host.yml"
    load_policy data "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-secrets.yml"
    load_policy conjur/authn-jwt "$POLICY_DIR/generated/$APP_NAMESPACE_NAME.conjur-authn.yml"
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

  if [ "$CONJUR_DEPLOYMENT" != "cloud" ]; then
    echo "Adding authn-azure variables"
    set_variable "conjur/authn-azure/$AUTHENTICATOR_ID/provider-uri" "$PROVIDER_URI"

    cli_exec logout
  fi
}

main "$@"
