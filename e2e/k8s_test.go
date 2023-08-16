//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"testing"
	"strings"
	"crypto/rand"
	"encoding/base64"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestSSHKeysProvidedK8s(t *testing.T) {
	f := features.New("ssh keys provided").
		// Replaces TEST_ID_1.1_providing_ssh_keys_successfully
		Assess("ssh key set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "SSH_KEY"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\"")
		
			return ctx
		})
	
	testenv.Test(t, f.Feature())
}

func TestJsonObjectSecretProvidedK8s(t *testing.T) {
	f := features.New("json object secret provided").
		// Replaces TEST_ID_1.2_providing_json_object_secret_successfully
		Assess("json object secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "JSON_OBJECT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\"")
		
			return ctx
		})
	
	testenv.Test(t, f.Feature())
}


func TestVariablesWithSpacesSecretProvidedK8s(t *testing.T) {
	f := features.New("variables with spaces secret provided").
		// Replaces TEST_ID_1.3_providing_variables_with_spaces_successfully
		Assess("variables with spaces secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_SPACES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")

			return ctx
		})
	
	testenv.Test(t, f.Feature())
}

func TestVariablesWithPlusesSecretProvidedK8s(t *testing.T) {
	f := features.New("variables with pluses secret provided").
		// Replaces TEST_ID_1.4_providing_variables_with_pluses_successfully
		Assess("variables with pluses secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_PLUSES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")
		
			return ctx
		})
	
	testenv.Test(t, f.Feature())
}

func TestSecretsProvidedK8s(t *testing.T) {
	f := features.New("secrets provided (K8s secrets mode)").
		// Replaces TEST_ID_1.5_providing_variables_with_german_umlaut_successfully
		Assess("umlaut secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_UMLAUT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "ÄäÖöÜü")

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestVariablesWithBase64DecodingSecretProvidedK8s(t *testing.T) {
	f := features.New("variables with base64 decoding secret provided").
		// Replaces TEST_ID_1.6_providing_variables_with_base64_decoding_successfully
		Assess("variables with base64 decoding secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "secret-value")

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestLargeDecodedVariablesSecretProvidedK8s(t *testing.T) {
	f := features.New("large decoded variables secret provided").
		// Replaces TEST_ID_1.7_providing_large_decoded_variable_successfully
		Assess("large decoded variable secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// generate random string (> 65k characters) and base64 encode it
			str := make([]byte, 65001)

			_, err := rand.Read(str)
			if err != nil {
				return err
			}

			encodedStr := base64.StdEncoding.EncodeToString(str)

			// set encoded value in conjur and reload template
			_, err := SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			if err != nil {
				return err
			}

			_, err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			if err != nil {
				return err
			}

			// check environment variable for expected value (the secret before encoding)
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), str)

			return ctx
		})

	testenv.Test(t, f.Feature())
}
