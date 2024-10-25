//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestSecretsProvidedK8s(t *testing.T) {
	f := features.New("secrets provided (K8s secrets mode)").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_1.1_providing_ssh_keys_successfully
		Assess("ssh key set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "SSH_KEY"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\"")

			return ctx
		}).
		// Replaces TEST_ID_1.2_providing_json_object_secret_successfully
		Assess("json object secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "JSON_OBJECT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\"")

			return ctx
		}).
		// Replaces TEST_ID_1.3_providing_variables_with_spaces_successfully
		Assess("variables with spaces secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_SPACES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")

			return ctx
		}).
		// Replaces TEST_ID_1.4_providing_variables_with_pluses_successfully
		Assess("variables with pluses secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_PLUSES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")

			return ctx
		}).
		// Replaces TEST_ID_1.5_providing_variables_with_german_umlaut_successfully
		Assess("umlaut secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer

			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_UMLAUT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "ÄäÖöÜü")

			return ctx
		}).
		// Replaces TEST_ID_1.6_providing_variables_with_base64_decoding_successfully
		Assess("variables with base64 decoding secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "secret-value")

			return ctx
		}).
		// Replaces TEST_ID_16_non_conjur_keys_stay_intact_in_k8s_secret
		Assess("non conjur keys stay intact secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "NON_CONJUR_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-value")

			return ctx
		}).
		Assess("fetch all secrets provided", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "FETCH_ALL_TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Assess("fetch all base64 secrets provided", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "FETCH_ALL_BASE64"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

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
			assert.Nil(t, err)

			err = ReloadWithTemplate(cfg.Client(), K8sTemplate)
			assert.Nil(t, err)

			return context.WithValue(ctx, "expected", string(str))
		}).
		// Replaces TEST_ID_1.7_providing_large_decoded_variable_successfully
		Assess("large decoded variable secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// check environment variable for expected value (the secret before encoding)
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), ctx.Value("expected").(string))

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset the secret value in Conjur
			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value"))
			err := SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestMultiplePodsChangingPwdInbetweenSecretProvidedK8s(t *testing.T) {
	f := features.New("multiple pods changing pwd inbetween").
		// Replaces TEST_ID_2_multiple_pods_changing_pwd_inbetween
		Assess("multiple pods secrets set correctly inbetween", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			// verify initial secret
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "supersecret")

			// scale and set secret to secret2
			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret2")
			assert.Nil(t, err)

			err = ScaleDeployment(cfg.Client(), "test-env", SecretsProviderNamespace(), SPLabelSelector, 0)
			assert.Nil(t, err)

			err = ScaleDeployment(cfg.Client(), "test-env", SecretsProviderNamespace(), SPLabelSelector, 1)
			assert.Nil(t, err)

			ClearBuffer(&stdout, &stderr)

			// verify new secret value in new pod
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret2")

			// scale and set secret to secret3
			err = SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret3")
			assert.Nil(t, err)

			err = ScaleDeployment(cfg.Client(), "test-env", SecretsProviderNamespace(), SPLabelSelector, 0)
			assert.Nil(t, err)

			err = ScaleDeployment(cfg.Client(), "test-env", SecretsProviderNamespace(), SPLabelSelector, 3)
			assert.Nil(t, err)

			pods, err := GetPods(cfg.Client(), SecretsProviderNamespace(), SPLabelSelector)
			assert.Nil(t, err)
			for _, pod := range pods.Items {
				ClearBuffer(&stdout, &stderr)

				// verify new secret value in new pods
				cfg.Client().Resources(SecretsProviderNamespace()).ExecInPod(context.TODO(), SecretsProviderNamespace(), pod.Name, TestAppContainer, command, &stdout, &stderr)
				assert.Contains(t, stdout.String(), "secret3")
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "supersecret")
			assert.Nil(t, err)

			err = ScaleDeployment(cfg.Client(), "test-env", SecretsProviderNamespace(), SPLabelSelector, 0)
			assert.Nil(t, err)

			err = ScaleDeployment(cfg.Client(), "test-env", SecretsProviderNamespace(), SPLabelSelector, 1)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHostNotInAppsSecretProvidedK8s(t *testing.T) {
	f := features.New("host not in apps secret provided").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set login configuration in local environment
			appNamespaceName := os.Getenv("APP_NAMESPACE_NAME")

			loginURI := fmt.Sprintf("host/some-apps/%s/*/*", appNamespaceName)

			os.Setenv("CONJUR_AUTHN_LOGIN", loginURI)

			// reload template with new login configuration
			err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_13_host_not_in_apps
		Assess("host not in apps secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// expect pod has conjur secret
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			os.Unsetenv("CONJUR_AUTHN_LOGIN")
			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHostInRootPolicySecretProvidedK8s(t *testing.T) {
	f := features.New("host in root policy secret provided").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set login configuration in local environment
			loginURI := fmt.Sprintf("host/%s/*/*", os.Getenv("APP_NAMESPACE_NAME"))

			os.Setenv("CONJUR_AUTHN_LOGIN", loginURI)

			// reload template with new login configuration
			err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_14_host_in_root_policy
		Assess("host in root policy secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// expect pod has conjur secret
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			os.Unsetenv("CONJUR_AUTHN_LOGIN")
			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHostWithApplicationIdentityInAnnotationsSecretProvidedK8s(t *testing.T) {
	f := features.New("host with application identity in annotations secret provided").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set login configuration in local environment
			loginURI := "host/some-apps/annotations-app"

			os.Setenv("CONJUR_AUTHN_LOGIN", loginURI)

			// reload template with new login configuration
			err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_15_host_with_application_identity_in_annotations
		Assess("host with application identity in annotations secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// expect pod has conjur secret
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			os.Unsetenv("CONJUR_AUTHN_LOGIN")
			return ctx
		})

	testenv.Test(t, f.Feature())
}
