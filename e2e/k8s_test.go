//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestSecretsProvidedK8s(t *testing.T) {
	f := features.New("secrets provided (K8s secrets mode)").
		Assess("ssh key set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "SSH_KEY"}

			RunCommandInSecretsProviderPod(cfg.Client(), spPodName, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\"")

			return ctx
		}).
		Assess("json object secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "JSON_OBJECT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), spPodName, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\"")

			return ctx
		}).
		Assess("variables with spaces secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_SPACES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), spPodName, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")

			return ctx
		}).
		Assess("variables with pluses secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_PLUSES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), spPodName, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")

			return ctx
		}).
		Assess("umlaut secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer

			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_UMLAUT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), spPodName, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "ÄäÖöÜü")

			return ctx
		}).
		Assess("variables with base64 decoding secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), spPodName, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "secret-value")

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestLargeDecodedVariableSecretProvidedK8s(t *testing.T) {
	f := features.New("large decoded variables secret provided").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			charSet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			length := 65001

			str := make([]byte, length)
			for i := 0; i < length; i++ {
				str[i] = charSet[rand.Intn(len(charSet))]
			}
			encodedStr := base64.StdEncoding.EncodeToString(str)

			// set encoded value in conjur and reload template
			err := SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			if err != nil {
				fmt.Errorf("error setting conjur secret: %s", err)
			}

			spPod, err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			if err != nil {
				fmt.Errorf("error reloading secrets provider pod: %s", err)
			}
			// Refresh the pod name after reload
			spPodName = spPod.Name

			return context.WithValue(ctx, "expected", string(str))
		}).
		Assess("large decoded variable secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// check environment variable for expected value (the secret before encoding)
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), spPodName, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), ctx.Value("expected").(string))

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset the secret value in Conjur
			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value"))
			err := SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			if err != nil {
				fmt.Errorf("error setting conjur secret: %s", err)
				return ctx
			}
			return ctx
		})

	testenv.Test(t, f.Feature())
}
