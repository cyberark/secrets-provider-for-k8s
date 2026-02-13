//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestPushToFile(t *testing.T) {
	f := features.New("push to file").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set secrets mode to P2F
			t.Setenv("SECRETS_MODE", "p2f")

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
		})

	testenv.Test(t, f.Feature())
}

func TestPushToFileSecretsRotation(t *testing.T) {
	f := features.New("push to file secrets rotation").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set conjur secret
			err := SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "secret1")
			assert.Nil(t, err)

			// set secrets mode to P2F rotation
			t.Setenv("SECRETS_MODE", "p2f-rotation")

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
			err := SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "secret2")
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value2"))
			err = SetConjurSecret(cfg.Client(), "data/secrets/encoded", encodedStr)
			assert.Nil(t, err)

			// wait for secrets to be rotated and files to be updated
			// Poll for the first file to confirm rotation has occurred
			err = WaitForFileContent(cfg.Client(), SPLabelSelector, TestAppContainer, "/opt/secrets/conjur/group5.template",
				"username | some-user\npassword | 7H1SiSmYp@5Sw0rd\ntest | secret2\n", 30*time.Second)
			assert.Nil(t, err)

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
				"group6.yaml": `"data/secrets/another_test_secret": "some-secret"
"data/secrets/encoded": "` + encodedStr + `"
"data/secrets/json_object_secret": "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\""
"data/secrets/password": "7H1SiSmYp@5Sw0rd"
"data/secrets/ssh_key": "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\""
"data/secrets/test_secret": "secret2"
"data/secrets/umlaut": "ÄäÖöÜü"
"data/secrets/url": "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
"data/secrets/username": "some-user"
"data/secrets/var with spaces": "some-secret"
"data/secrets/var+with+pluses": "some-secret"`,
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

			// wait for files to be deleted after secret deletion
			// Poll for the first file to be deleted
			err = WaitForFileDeleted(cfg.Client(), SPLabelSelector, TestAppContainer, "/opt/secrets/conjur/group1.yaml", 30*time.Second)
			assert.Nil(t, err)

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
			// reset conjur secrets
			err := RestoreTestSecret(cfg.Client())
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value"))
			err = SetConjurSecret(cfg.Client(), "data/secrets/encoded", encodedStr)
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
			t.Setenv("SECRETS_MODE", "k8s-rotation")

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
		Assess("initial values are set correctly", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			// verify initial secret values
			err := WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "supersecret", 20*time.Second)
			assert.Nil(t, err)

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "var_with_base64", "secret-value", 20*time.Second)
			assert.Nil(t, err)

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all", "data.secrets.test_secret", "supersecret", 20*time.Second)
			assert.Nil(t, err)

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all-base64", "data.secrets.encoded", "secret-value", 20*time.Second)
			assert.Nil(t, err)

			return ctx
		}).
		Assess("values are updated correctly", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// change conjur secrets
			err := SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "secret2")
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value2"))
			err = SetConjurSecret(cfg.Client(), "data/secrets/encoded", encodedStr)
			assert.Nil(t, err)

			// wait for K8s secrets to be rotated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "secret2", 20*time.Second)
			assert.Nil(t, err)

			// verify VARIABLE_WITH_BASE64_SECRET updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "var_with_base64", "secret-value2", 20*time.Second)
			assert.Nil(t, err)

			// verify FETCH_ALL_TEST_SECRET updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all", "data.secrets.test_secret", "secret2", 20*time.Second)
			assert.Nil(t, err)

			// verify FETCH_ALL_BASE64 updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all-base64", "data.secrets.encoded", "secret-value2", 20*time.Second)
			assert.Nil(t, err)

			return ctx
		}).
		Assess("values are removed correctly", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			// delete a secret from conjur
			err := DeleteTestSecret(cfg.Client())
			require.Nil(t, err)

			// wait for secret to be removed from K8s after deletion
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "", 20*time.Second)
			assert.Nil(t, err)

			// Verify FETCH_ALL_BASE64 still contains secret-value2
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all-base64", "data.secrets.encoded", "secret-value2", 20*time.Second)
			assert.Nil(t, err)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset conjur secrets
			err := RestoreTestSecret(cfg.Client())
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value"))
			err = SetConjurSecret(cfg.Client(), "data/secrets/encoded", encodedStr)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestLabeledK8sSecretsRotation(t *testing.T) {
	f := features.New("timer-based k8s secrets rotation with labeled secrets").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set secrets mode to K8s Rotation
			t.Setenv("SECRETS_MODE", "k8s-rotation")
			t.Setenv("LABELED_SECRETS", "true")
			t.Setenv("SECRETS_REFRESH_INTERVAL", "5s")

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
		Assess("initial values are set correctly", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify initial secret values
			err := WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "supersecret", 20*time.Second)
			assert.Nil(t, err)

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "var_with_base64", "secret-value", 20*time.Second)
			assert.Nil(t, err)

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all", "data.secrets.test_secret", "supersecret", 20*time.Second)
			assert.Nil(t, err)

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all-base64", "data.secrets.encoded", "secret-value", 20*time.Second)
			assert.Nil(t, err)

			return ctx
		}).
		Assess("values are updated correctly", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// change conjur secrets
			err := SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "secret2")
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value2"))
			err = SetConjurSecret(cfg.Client(), "data/secrets/encoded", encodedStr)
			assert.Nil(t, err)

			// verify VARIABLE_WITH_BASE64_SECRET updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "var_with_base64", "secret-value2", 20*time.Second)
			assert.Nil(t, err)

			// verify FETCH_ALL_TEST_SECRET updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all", "data.secrets.test_secret", "secret2", 20*time.Second)
			assert.Nil(t, err)

			// verify FETCH_ALL_BASE64 updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all-base64", "data.secrets.encoded", "secret-value2", 20*time.Second)
			assert.Nil(t, err)

			return ctx
		}).
		Assess("values are removed correctly", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// delete a secret from conjur
			err := DeleteTestSecret(cfg.Client())
			require.Nil(t, err)

			// wait for secret to be removed from K8s after deletion
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "", 20*time.Second)
			assert.Nil(t, err)

			// Verify secret value is removed from fetch all secret
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all", "data.secrets.test_secret", "", 20*time.Second)
			assert.Nil(t, err)

			// Verify FETCH_ALL_BASE64 still contains secret-value2
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-fetch-all-base64", "data.secrets.encoded", "secret-value2", 20*time.Second)
			assert.Nil(t, err)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// reset conjur secrets
			err := RestoreTestSecret(cfg.Client())
			assert.Nil(t, err)

			encodedStr := base64.StdEncoding.EncodeToString([]byte("secret-value"))
			err = SetConjurSecret(cfg.Client(), "data/secrets/encoded", encodedStr)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestLabeledK8sSecretsRotationViaInformer(t *testing.T) {
	f := features.New("informer-based k8s secrets rotation with labeled secrets").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set secrets mode to K8s Rotation
			t.Setenv("SECRETS_MODE", "k8s-rotation")
			t.Setenv("LABELED_SECRETS", "true")
			// Set a long interval so we can test informer-triggered updates
			t.Setenv("SECRETS_REFRESH_INTERVAL", "999m")

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
		Assess("new labeled secret is processed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create a new K8s secret with the proper label and conjur-map
			secret := corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-k8s-secret",
					Namespace: SecretsProviderNamespace(),
					Labels: map[string]string{
						"conjur.org/managed-by-provider": "true",
					},
					Annotations: map[string]string{
						"conjur.org/conjur-secrets.example.1": `- username: data/secrets/username
- password: data/secrets/password
- test: data/secrets/test_secret`,
						"conjur.org/secret-file-template.example.1": `username | {{ secret "username" }}
password | {{ secret "password" }}
test | {{ secret "test" }}`,
					},
				},
				StringData: map[string]string{
					"conjur-map": "NEW_SECRET: data/secrets/another_test_secret",
				},
				Type: "Opaque",
			}
			err := cfg.Client().Resources(SecretsProviderNamespace()).Create(context.TODO(), &secret)
			require.Nil(t, err, "Failed to create new labeled K8s secret")

			// Wait for the secret to be updated in Kubernetes (check the actual secret data)
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "labeled-k8s-secret", "NEW_SECRET", "some-secret", 20*time.Second)
			assert.Nil(t, err)

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "labeled-k8s-secret", "example.1", "username | some-user\npassword | 7H1SiSmYp@5Sw0rd\ntest | supersecret", 20*time.Second)
			assert.Nil(t, err)

			return ctx
		}).
		Assess("updated labeled secret is processed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			secret := corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-k8s-secret",
					Namespace: SecretsProviderNamespace(),
					Labels: map[string]string{
						"conjur.org/managed-by-provider": "true",
					},
					Annotations: map[string]string{
						"conjur.org/conjur-secrets.example.1":       `- test_secret: data/secrets/test_secret`,
						"conjur.org/secret-file-template.example.1": `test_secret | {{ secret "test_secret" }}`,
					},
				},
				StringData: map[string]string{
					"conjur-map": "secret1: data/secrets/another_test_secret\nsecret2: data/secrets/password",
				},
				Type: "Opaque",
			}
			err := cfg.Client().Resources(SecretsProviderNamespace()).Update(context.TODO(), &secret)
			require.Nil(t, err, "Failed to update labeled K8s secret")

			// Wait for both keys to be populated in the secret data
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "labeled-k8s-secret", "secret1", "some-secret", 20*time.Second)
			assert.Nil(t, err, "secret1 should be populated")

			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "labeled-k8s-secret", "secret2", "7H1SiSmYp@5Sw0rd", 20*time.Second)
			assert.Nil(t, err, "secret2 should be populated")

			return ctx
		}).
		Assess("remove one key from conjur-map and verify cleanup", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Update the secret to remove secret2 from conjur-map
			secret := corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-k8s-secret",
					Namespace: SecretsProviderNamespace(),
					Labels: map[string]string{
						"conjur.org/managed-by-provider": "true",
					},
				},
				StringData: map[string]string{
					"conjur-map": "secret1: data/secrets/another_test_secret",
				},
				Type: "Opaque",
			}
			err := cfg.Client().Resources(SecretsProviderNamespace()).Update(context.TODO(), &secret)
			require.Nil(t, err, "Failed to update cleanup test secret")

			// Wait for secret1 to be re-populated (confirming informer processed the update)
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "labeled-k8s-secret", "secret1", "some-secret", 20*time.Second)
			assert.Nil(t, err, "secret1 should still exist with correct value after update")

			// Verify that secret2 key has been removed from the secret data
			updatedSecret, err := GetSecret(cfg.Client(), "labeled-k8s-secret")
			require.Nil(t, err, "Should be able to retrieve the updated secret")

			_, secret2Exists := updatedSecret.Data["secret2"]
			assert.False(t, secret2Exists, "secret2 key should be removed from secret data after conjur-map update")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := DeleteSecret(cfg.Client(), "labeled-k8s-secret")
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}
