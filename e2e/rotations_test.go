//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestPushToFile(t *testing.T) {
	f := features.New("push to file").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set secrets mode to P2F
			os.Setenv("SECRETS_MODE", "p2f")

			// reload testing environment with P2F template
			err := ReloadWithTemplate(cfg.Client(), P2fTemplate)
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_27_push_to_file
		Assess("p2f", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// files to test against
			files := []string{"group1.yaml", "group2.json", "some-dotenv.env", "group4.bash", "group5.template", "group6.json", "group7.template", "group8.yaml"}

			// expected content in files
			expectedContent := map[string]string{
				"group1.yaml": "\"url\": \"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n" +
					"\"username\": \"some-user\"\n" +
					"\"password\": \"7H1SiSmYp@5Sw0rd\"\n" +
					"\"encoded\": \"secret-value\"",
				"group2.json": "{\"url\":\"postgresql://test-app-backend.app-test.svc.cluster.local:5432\",\"username\":\"some-user\",\"password\":\"7H1SiSmYp@5Sw0rd\",\"still_encoded\":\"c2VjcmV0LXZhbHVl\"}",
				"some-dotenv.env": "url=\"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n" +
					"username=\"some-user\"\n" +
					"password=\"7H1SiSmYp@5Sw0rd\"",
				"group4.bash": "export url=\"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n" +
					"export username=\"some-user\"\n" +
					"export password=\"7H1SiSmYp@5Sw0rd\"",
				"group5.template": "username | some-user\n" +
					"password | 7H1SiSmYp@5Sw0rd\n",
				"group6.json":     FetchAllJSONContent,
				"group7.template": FetchAllBase64TemplateContent,
				"group8.yaml":     FetchAllBase64YamlContent,
			}

			// check file contents match container output
			var stdout, stderr bytes.Buffer
			for _, f := range files {
				fmt.Printf("Checking file %s content, file format: %s\n", f, filepath.Ext(f))

				ClearBuffer(&stdout, &stderr)
				command := []string{"cat", fmt.Sprintf("/opt/secrets/conjur/%s", f)}
				RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

				assert.Equal(t, expectedContent[f], stdout.String())
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset environment variable to default k8s
			os.Setenv("SECRETS_MODE", "k8s")

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestPushToFileSecretsRotation(t *testing.T) {
	f := features.New("push to file secrets rotation").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set conjur secret
			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret1")
			assert.Nil(t, err)

			// set secrets mode to P2F rotation
			os.Setenv("SECRETS_MODE", "p2f-rotation")

			// reload testing environment with P2F rotation template
			err = ReloadWithTemplate(cfg.Client(), P2fRotationTemplate)
			assert.Nil(t, err)

			// expect 2 containers since we're using a sidecar
			pod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), SPLabelSelector)
			assert.Nil(t, err)

			assert.Equal(t, 2, len(pod.Spec.Containers))

			// delete any initial 'generated' and 'policy' directories
			err = DeleteTestingDirectories(cfg.Client())
			assert.Nil(t, err)

			// create temporary 'generated' and 'policy' directories for testing
			err = CreateTestingDirectories(cfg.Client())
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_28_push_to_file_secrets_rotation
		Assess("p2f rotation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// change a conjur variable
			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret2")
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value2"))
			err = SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			assert.Nil(t, err)

			// wait for value to refresh in conjur
			time.Sleep(15 * time.Second)

			// files to test against
			files := []string{"group1.yaml", "group2.json", "some-dotenv.env", "group4.bash", "group5.template", "group6.yaml"}

			// expected content in files
			expectedContent := map[string]string{
				"group1.yaml": "\"url\": \"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n" +
					"\"username\": \"some-user\"\n" +
					"\"password\": \"7H1SiSmYp@5Sw0rd\"\n" +
					"\"test\": \"secret2\"\n" +
					"\"encoded\": \"secret-value2\"",
				"group2.json": "{\"url\":\"postgresql://test-app-backend.app-test.svc.cluster.local:5432\",\"username\":\"some-user\",\"password\":\"7H1SiSmYp@5Sw0rd\",\"test\":\"secret2\",\"still_encoded\":\"c2VjcmV0LXZhbHVlMg==\"}",
				"some-dotenv.env": "url=\"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n" +
					"username=\"some-user\"\n" +
					"password=\"7H1SiSmYp@5Sw0rd\"\n" +
					"test=\"secret2\"",
				"group4.bash": "export url=\"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n" +
					"export username=\"some-user\"\n" +
					"export password=\"7H1SiSmYp@5Sw0rd\"\n" +
					"export test=\"secret2\"",
				"group5.template": "username | some-user\n" +
					"password | 7H1SiSmYp@5Sw0rd\n" +
					"test | secret2\n",
				"group6.yaml": `"secrets/another_test_secret": "some-secret"
"secrets/encoded": "` + encodedStr + `"
"secrets/json_object_secret": "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\""
"secrets/password": "7H1SiSmYp@5Sw0rd"
"secrets/ssh_key": "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\""
"secrets/test_secret": "secret2"
"secrets/umlaut": "ÄäÖöÜü"
"secrets/url": "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
"secrets/username": "some-user"
"secrets/var with spaces": "some-secret"
"secrets/var+with+pluses": "some-secret"`,
			}

			// check file contents match container output
			var stdout, stderr bytes.Buffer
			for _, f := range files {
				fmt.Printf("Checking file %s content, file format: %s\n", f, filepath.Ext(f))

				ClearBuffer(&stdout, &stderr)
				command := []string{"cat", fmt.Sprintf("/opt/secrets/conjur/%s", f)}
				RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

				assert.Equal(t, expectedContent[f], stdout.String())
			}

			// delete a secret from conjur
			err = DeleteTestSecret(cfg.Client())
			require.Nil(t, err)

			// wait for values to be deleted in conjur
			time.Sleep(15 * time.Second)

			for _, f := range files {
				fmt.Printf("Checking if file %s exists\n", f)

				ClearBuffer(&stdout, &stderr)
				command := []string{"cat", fmt.Sprintf("/opt/secrets/conjur/%s", f)}
				RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)

				assert.Equal(t, "", stdout.String())
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset environment variable to default k8s
			os.Setenv("SECRETS_MODE", "k8s")

			// reset conjur secrets
			err := RestoreTestSecret(cfg.Client())
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value"))
			err = SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			assert.Nil(t, err)

			// delete temporary 'generated' and 'policy' directories
			err = DeleteTestingDirectories(cfg.Client())
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestK8sSecretsRotation(t *testing.T) {
	f := features.New("k8s secrets rotation").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set secrets mode to K8s Rotation
			os.Setenv("SECRETS_MODE", "k8s-rotation")

			// reload testing environment with K8s Rotation template
			err := ReloadWithTemplate(cfg.Client(), K8sRotationTemplate)
			assert.Nil(t, err)

			// delete any initial 'generated' and 'policy' directories
			err = DeleteTestingDirectories(cfg.Client())
			assert.Nil(t, err)

			// create temporary 'generated' and 'policy' directories for testing
			err = CreateTestingDirectories(cfg.Client())
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_29_k8s_secrets_rotation
		Assess("k8s rotation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// // verify initial secret values
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "supersecret")
			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret-value")
			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "FETCH_ALL_TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "supersecret")
			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "FETCH_ALL_BASE64"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret-value")
			ClearBuffer(&stdout, &stderr)

			// change conjur secrets
			err := SetConjurSecret(cfg.Client(), "secrets/test_secret", "secret2")
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value2"))
			err = SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			assert.Nil(t, err)

			// wait for conjur secrets to refresh
			time.Sleep(45 * time.Second)

			// verify new secret values
			command = []string{"printenv", "|", "grep", "TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret2")
			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "VARIABLE_WITH_BASE64_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret-value2")
			ClearBuffer(&stdout, &stderr)

			// verify new secret values
			command = []string{"printenv", "|", "grep", "FETCH_ALL_TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret2")
			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "FETCH_ALL_BASE64"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret-value2")
			ClearBuffer(&stdout, &stderr)

			// delete a secret from conjur
			err = DeleteTestSecret(cfg.Client())
			require.Nil(t, err)

			// wait for values to be deleted in conjur
			time.Sleep(45 * time.Second)

			command = []string{"printenv", "|", "grep", "TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Empty(t, strings.TrimSpace(stdout.String()))
			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "FETCH_ALL_TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Empty(t, strings.TrimSpace(stdout.String()))
			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "FETCH_ALL_BASE64"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "secret-value2") // This one should still be there

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset environment variable to default k8s
			os.Setenv("SECRETS_MODE", "k8s")

			// reset conjur secrets
			err := RestoreTestSecret(cfg.Client())
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value"))
			err = SetConjurSecret(cfg.Client(), "secrets/encoded", encodedStr)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}
