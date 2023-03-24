#!/bin/bash
set -euxo pipefail

create_secret_access_role

create_secret_access_role_binding

# Variables to set in Conjur
var_arr=("secrets/ssh" \
          "secrets/json" \
          "secrets/var with spaces" \
          "secrets/var+with+pluses" \
          "secrets/umlaut" )

# Values to push to Conjur (should match up with variables above)
values_arr=( "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\"" \
          "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\"" \
          "var_with_spaces_secret" \
          "var_with_pluses_secret" \
          "ÄäÖöÜü" )

# Environment variables to set in test app via Secrets Provider
env_var_arr=( "VARIABLE_WITH_SSH_SECRET" \
              "VARIABLE_WITH_JSON_SECRET" \
              "VARIABLE_WITH_SPACES_SECRET" \
              "VARIABLE_WITH_PLUSES_SECRET" \
              "VARIABLE_WITH_UMLAUT_SECRET" )

# Set Conjur variables
for i in "${!var_arr[@]}"
do  
    var="${var_arr[$i]}"
    value="${values_arr[$i]}"
    set_conjur_secret "$var" "$value"
done

# Deploy test app
set_namespace "$APP_NAMESPACE_NAME"
deploy_env
pod_name="$(get_pod_name "$APP_NAMESPACE_NAME" 'app=test-env')"

# Verify environment variables set correctly in pod
for i in "${!env_var_arr[@]}"
do  
    env_var="${env_var_arr[$i]}"
    secret_value="${values_arr[$i]}"

    verify_secret_value_in_pod "$pod_name" "$env_var" "$expected_value"
done
