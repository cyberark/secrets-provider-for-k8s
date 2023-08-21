//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"fmt"
	"bytes"
	"context"
	"os/exec"
	"testing"
	"math/rand"
	"encoding/base64"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestSecretsProvidedK8s(t *testing.T) {
	f := features.New("secrets provided (K8s secrets mode)").
		// Replaces TEST_ID_1.1_providing_ssh_keys_successfully
		Assess("ssh key set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "SSH_KEY"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\"")

			return ctx
		}).
		// Replaces TEST_ID_1.2_providing_json_object_secret_successfully
		Assess("json object secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "JSON_OBJECT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\"")

			return ctx
		}).
		// Replaces TEST_ID_1.3_providing_variables_with_spaces_successfully
		Assess("variables with spaces secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_SPACES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")

			return ctx
		}).
		// Replaces TEST_ID_1.4_providing_variables_with_pluses_successfully
		Assess("variables with pluses secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_PLUSES_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-secret")

			return ctx
		}).
		// Replaces TEST_ID_1.5_providing_variables_with_german_umlaut_successfully
		Assess("umlaut secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer

			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_UMLAUT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "ÄäÖöÜü")

			return ctx
		}).
		// Replaces TEST_ID_1.6_providing_variables_with_base64_decoding_successfully
		Assess("variables with base64 decoding secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "secret-value")

			return ctx
		}).
		// Replaces TEST_ID_16_non_conjur_keys_stay_intact_in_k8s_secret
		Assess("non conjur keys stay intact secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "NON_CONJUR_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "some-value")

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

			err = ReloadWithTemplate(cfg.Client(), K8sTemplate)
			if err != nil {
				fmt.Errorf("error reloading secrets provider pod: %s", err)
			}

			return context.WithValue(ctx, "expected", string(str))
		}).
		// Replaces TEST_ID_1.7_providing_large_decoded_variable_successfully
		Assess("large decoded variable secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// check environment variable for expected value (the secret before encoding)
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

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

func TestMultiplePodsChangingPwdInbetweenSecretProvidedK8s(t *testing.T) {
	f := features.New("multiple pods changing pwd inbetween").
		// Replaces TEST_ID_2_multiple_pods_changing_pwd_inbetween
		Assess("secret set correctly in pod1", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			platform := os.Getenv("PLATFORM")

			if platform == "kubernetes" {
				err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret2")
				if err != nil {
					fmt.Errorf("error setting conjur secret: %s", err)
				}
			} else if platform == "openshift" {
				fmt.Println("else statement")
				err := SetConjurSecret(cfg.Client(), "TEST_SECRET", "secret2")
				if err != nil {
					fmt.Errorf("error setting conjur secret: %s", err)
				}
			}

			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret2")
			if err != nil {
				fmt.Errorf("error setting conjur secret: %s", err)
			}

			err = ReloadWithTemplate(cfg.Client(), K8sTemplate)
			if err != nil {
				fmt.Errorf("error reloading secrets provider pod: %s", err)
			}

			return ctx
		}).
		Assess("secret set correctly in pod2", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "secret2")

			return ctx
		}).
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret3")
			if err != nil {
				fmt.Errorf("error setting conjur secret: %s", err)
			}

			err = ReloadWithTemplate(cfg.Client(), K8sTemplate)
			if err != nil {
				fmt.Errorf("error reloading secrets provider pod: %s", err)
			}

			return ctx
		}).
		Assess("secret set correctly in pod3", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "secret3")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset the secret value in Conjur
			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "supersecret")
			if err != nil {
				fmt.Errorf("error setting conjur secret: %s", err)
			}
			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestNoPermissionSecretProvidedK8s(t *testing.T) {
	f := features.New("no permission to view conjur secrets").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set login configuration in local environment
			authenticatorID := os.Getenv("AUTHENTICATOR_ID")
			appNamespaceName := os.Getenv("APP_NAMESPACE_NAME")

			loginURI := fmt.Sprintf("host/conjur/authn-k8s/%s/apps/%s/service_account/%s-sa", authenticatorID, appNamespaceName, appNamespaceName)

			os.Setenv("CONJUR_AUTHN_LOGIN", loginURI)

			// In this case the pod will fail to start due to the expected error in Secrets Provider.
			// Currently ReloadWithTemplate has a built-in 1 minute timeout for the pod to be 'Ready'
			// We may be able to shorten or omit this timeout to speed up the test.
			err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			if err != nil {
				fmt.Errorf("error reloading secrets provider: %s", err)
			}
			return ctx
		}).
		// Replaces TEST_ID_12_no_conjur_secrets_permission
		Assess("no permission to view conjur secrets in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			spPod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), SecretsProviderLabelSelector)

			getPodLogsCommand := exec.Command("kubectl", "logs", spPod.Name, "-c", "cyberark-secrets-provider-for-k8s")
			podLogs, err := getPodLogsCommand.CombinedOutput()
			if err != nil {
				fmt.Errorf("error getting pod logs output: %s", err)
			}

			assert.Contains(t, string(podLogs), "CSPFK034E") // for reference, a successful conjur secrets update: CSPFK009I

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			os.Unsetenv("CONJUR_AUTHN_LOGIN")
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
			if err != nil {
				fmt.Errorf("error reloading secrets provider: %s", err)
			}
			return ctx
		}).
		// Replaces TEST_ID_13_host_not_in_apps
		Assess("host not in apps secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// expect pod has conjur secret
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

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
			if err != nil {
				fmt.Errorf("error reloading secrets provider: %s", err)
			}
			return ctx
		}).
		// Replaces TEST_ID_14_host_in_root_policy
		Assess("host in root policy secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// expect pod has conjur secret
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

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
			if err != nil {
				fmt.Errorf("error reloading secrets provider: %s", err)
			}
			return ctx
		}).
		// Replaces TEST_ID_15_host_with_application_identity_in_annotations
		Assess("host with application identity in annotations secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// expect pod has conjur secret
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			os.Unsetenv("CONJUR_AUTHN_LOGIN")
			return ctx
		})

	testenv.Test(t, f.Feature())
}
