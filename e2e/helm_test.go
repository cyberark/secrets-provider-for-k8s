//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestHelmJobDeploysSuccessfully(t *testing.T) {
	f := features.New("helm deploy").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// removing Init container for testing
			deployment, err := GetDeployment(cfg.Client(), "test-env")
			assert.Nil(t, err)

			err = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			if err != nil {
				fmt.Printf("WARN: %s", err)
			}

			// set up job and test app to test against
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			if err != nil {
				fmt.Printf("WARN: %s", err)
			}

			err = DeployTestAppWithHelm(cfg.Client(), "")
			assert.Nil(t, err)

			// wait for value to refresh in conjur
			time.Sleep(15 * time.Second)

			return ctx
		}).
		// Replaces TEST_ID_17_helm_job_deploys_successfully
		Assess("helm job deploys successfully", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify secret value in test app pod
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "supersecret")

			ClearBuffer(&stdout, &stderr)

			// verify all secrets were updated successfully in secrets-provider pod
			pod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), SPHelmLabelSelector)
			assert.Nil(t, err)

			logs, err := GetPodLogs(cfg.Client(), pod.Name, SecretsProviderNamespace(), LogsContainer)
			assert.Nil(t, err)

			assert.Contains(t, logs.String(), "CSPFK009I DAP/Conjur Secrets updated in Kubernetes successfully")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up job and test app
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			deployment, err := GetDeployment(cfg.Client(), "test-env")
			assert.Nil(t, err)

			err = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestMultipleHelmJobsMultipleSecret(t *testing.T) {
	f := features.New("multiple helm multiple secret").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// create second secret and set to a different value
			err := CreateK8sSecretForHelmDeployment(cfg.Client())
			assert.Nil(t, err)

			err = SetConjurSecret(cfg.Client(), "secrets/another_test_secret", "another-some-secret-value")
			assert.Nil(t, err)

			// set up first Secrets Provider job and deploy first test app
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			assert.Nil(t, err)

			err = DeployTestAppWithHelm(cfg.Client(), "")
			assert.Nil(t, err)

			// set up second Secrets Provider job and deploy second test app
			chartPath, err = FillHelmChart(cfg.Client(), "another-", map[string]string{
				"SECRETS_PROVIDER_ROLE":           "another-secrets-provider-role",
				"SECRETS_PROVIDER_ROLE_BINDING":   "another-secrets-provider-role-binding",
				"SERVICE_ACCOUNT":                 "another-secrets-provider-service-account",
				"K8S_SECRETS":                     "another-test-k8s-secret",
				"SECRETS_PROVIDER_SSL_CONFIG_MAP": "another-secrets-provider-ssl-config-map",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "another-", chartPath)
			assert.Nil(t, err)

			err = DeployTestAppWithHelm(cfg.Client(), "another-")
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_18_helm_multiple_provider_multiple_secrets
		Assess("multiple providers multiple secret", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify secret values in test app pods
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "supersecret")

			ClearBuffer(&stdout, &stderr)

			command = []string{"printenv", "|", "grep", "another-TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), "app=another-test-env", "another-test-app", command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "another-some-secret-value")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// delete created secret
			err := DeleteSecret(cfg.Client(), "another-test-k8s-secret")
			assert.Nil(t, err)

			// clean up jobs
			err = RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			err = RemoveJobWithHelm(cfg, "another-secrets-provider")
			assert.Nil(t, err)

			// clean up test apps
			dep1, err := GetDeployment(cfg.Client(), "test-env")
			assert.Nil(t, err)

			err = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), dep1)
			assert.Nil(t, err)

			dep2, err := GetDeployment(cfg.Client(), "another-test-env")
			assert.Nil(t, err)

			err = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), dep2)
			assert.Nil(t, err)

			// reset secret value
			err = SetConjurSecret(cfg.Client(), "secrets/another_test_secret", "some-secret")
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestMultipleHelmJobsSameSecret(t *testing.T) {
	f := features.New("multiple helm same secret").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set up jobs
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			assert.Nil(t, err)

			chartPath, err = FillHelmChart(cfg.Client(), "another-", map[string]string{
				"SECRETS_PROVIDER_ROLE":           "another-secrets-provider-role",
				"SECRETS_PROVIDER_ROLE_BINDING":   "another-secrets-provider-role-binding",
				"SERVICE_ACCOUNT":                 "another-secrets-provider-service-account",
				"SECRETS_PROVIDER_SSL_CONFIG_MAP": "another-secrets-provider-ssl-config-map",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "another-", chartPath)
			assert.Nil(t, err)

			// deploy test app to test against
			err = DeployTestAppWithHelm(cfg.Client(), "")
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_19_helm_multiple_provider_same_secret
		Assess("multiple providers same secret", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify secret value in test app pod
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up jobs
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			err = RemoveJobWithHelm(cfg, "another-secrets-provider")
			assert.Nil(t, err)

			// clean up test app
			deployment, err := GetDeployment(cfg.Client(), "test-env")
			assert.Nil(t, err)

			err = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestMultipleHelmJobsSameServiceAccount(t *testing.T) {
	t.Skip("Temporarily skipping due to flakiness holding up release builds. This test probably needs to be rewritten.")
	f := features.New("multiple helm same service account").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set up jobs
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			assert.Nil(t, err)

			chartPath, err = FillHelmChart(cfg.Client(), "another-", map[string]string{
				"CREATE_SERVICE_ACCOUNT":          "false",
				"LABELS":                          "app: another-test-helm",
				"SERVICE_ACCOUNT":                 "secrets-provider-service-account",
				"SECRETS_PROVIDER_SSL_CONFIG_MAP": "another-secrets-provider-ssl-config-map",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "another-", chartPath)
			assert.Nil(t, err)

			// deploy test app to test against
			err = DeployTestAppWithHelm(cfg.Client(), "")
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_20_helm_multiple_provider_same_serviceaccount
		Assess("multiple providers same service account", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify that another-secrets-provider runs with the correct service account
			var stdout, stderr bytes.Buffer
			pod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), "app=another-test-helm")
			assert.Nil(t, err)

			assert.Contains(t, pod.Spec.ServiceAccountName, "secrets-provider-service-account")

			ClearBuffer(&stdout, &stderr)

			// verify secret value in test app pod
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}
			RunCommandInSecretsProviderPod(cfg.Client(), SPLabelSelector, TestAppContainer, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up jobs
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			err = RemoveJobWithHelm(cfg, "another-secrets-provider")
			assert.Nil(t, err)

			// clean up test app
			deployment, err := GetDeployment(cfg.Client(), "test-env")
			assert.Nil(t, err)

			err = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHelmRbacDefaultsSuccessful(t *testing.T) {
	f := features.New("helm rbac defaults successful").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set up job
			chartPath, err := FillHelmChartNoOverrideDefaults(cfg.Client(), "", map[string]string{
				"LOG_LEVEL":          "debug",
				"LABELS":             "app: test-helm",
				"K8S_SECRETS":        "test-k8s-secret",
				"CONJUR_ACCOUNT":     ConjurAccount(),
				"CONJUR_AUTHN_URL":   ConjurAuthnUrl(),
				"CONJUR_AUTHN_LOGIN": "host/conjur/authn-k8s/" + AuthenticatorId() + "/apps/" + SecretsProviderNamespace() + "/*/*",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_22_helm_rbac_defaults_taken_successfully
		Assess("helm rbac defaults successful", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify that known defaults were taken if not supplied
			sa, err := GetServiceAccount(cfg.Client(), "secrets-provider-service-account")
			assert.Nil(t, err)

			role, roleBinding, err := GetRoleAndBinding(cfg.Client(), "secrets-provider-role", "secrets-provider-role-binding")
			assert.Nil(t, err)

			configMap, err := GetConfigMap(cfg.Client(), "cert-config-map")
			assert.Nil(t, err)

			assert.Equal(t, "secrets-provider-service-account", sa.Name)
			assert.Equal(t, "secrets-provider-role", role.Name)
			assert.Equal(t, "secrets-provider-role-binding", roleBinding.Name)
			assert.Equal(t, "cert-config-map", configMap.Name)

			// verify that the Secrets Provider took the default image configuration if not supplied & deployed successfully
			job, err := GetJob(cfg.Client(), "secrets-provider")
			assert.Nil(t, err)

			assert.Contains(t, job.Spec.Template.Spec.Containers[0].Image, "cyberark/secrets-provider-for-k8s:latest")
			assert.Equal(t, int32(1), job.Status.Succeeded)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up job
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHelmServiceAccountExists(t *testing.T) {
	f := features.New("helm service account exists").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// create K8s role
			err := CreateK8sRole(cfg.Client(), "another-")
			assert.Nil(t, err)

			// set up job
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{
				// RBAC should not be created --> thus roles should not be created
				"CREATE_SERVICE_ACCOUNT": "false",
				"SERVICE_ACCOUNT":        "another-secrets-provider-service-account",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			assert.Nil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_23_helm_service_account_exists
		Assess("helm service account exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify that resources were not created
			sa, err := GetServiceAccount(cfg.Client(), "secrets-provider-service-account")
			assert.NotNil(t, err)

			role, roleBinding, err := GetRoleAndBinding(cfg.Client(), "secrets-provider-role", "secrets-provider-role-binding")
			assert.NotNil(t, err)

			assert.Equal(t, "", sa.Name)
			assert.Equal(t, "", role.Name)
			assert.Equal(t, "", roleBinding.Name)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up job
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			// delete resources created from CreateK8sRole
			err = DeleteServiceAccount(cfg.Client(), "another-secrets-provider-service-account")
			assert.Nil(t, err)

			err = DeleteRoleAndBinding(cfg.Client(), "another-secrets-provider-role", "another-secrets-provider-role-binding")
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHelmValidateK8sSecret(t *testing.T) {
	f := features.New("helm validate k8s secret incorrect").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set up job
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{
				"K8S_SECRETS": "K8S_SECRET-non-existent-secret",
				"LABELS":      "app: test-helm",
				"LOG_LEVEL":   "debug",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			assert.NotNil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_24_helm_validate_K8S_SECRETS_env_var_incorrect_value
		Assess("helm validate k8s secret incorrect", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify secrets provider fails
			pod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), SPHelmLabelSelector)
			assert.Nil(t, err)

			logs, err := GetPodLogs(cfg.Client(), pod.Name, SecretsProviderNamespace(), LogsContainer)
			assert.Nil(t, err)

			assert.Contains(t, logs.String(), "CSPFK004D Failed to retrieve Kubernetes Secret")
			assert.Contains(t, logs.String(), "CSPFK020E Failed to retrieve Kubernetes Secret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up job
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHelmDefaultRetrySuccessful(t *testing.T) {
	f := features.New("helm default retry successful").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set up job
			imageChartPath, err := FillHelmChartTestImage(cfg.Client(), "", map[string]string{
				"IMAGE":             GetImagePath() + "/secrets-provider",
				"IMAGE_PULL_POLICY": "IfNotPresent",
				"TAG":               "latest",
			})
			assert.Nil(t, err)

			chartPath, err := FillHelmChartNoOverrideDefaults(cfg.Client(), "", map[string]string{
				"LABELS":               "app: test-helm",
				"LOG_LEVEL":            "debug",
				"K8S_SECRETS":          "test-k8s-secret",
				"CONJUR_ACCOUNT":       ConjurAccount(),
				"CONJUR_APPLIANCE_URL": ConjurApplianceUrl(),
				"CONJUR_AUTHN_LOGIN":   "host/conjur/authn-k8s/" + AuthenticatorId() + "/apps/" + SecretsProviderNamespace() + "/*/*",
				// parameter that will force failure
				"CONJUR_AUTHN_URL":   ConjurAuthnUrl() + "xyz",
				"RETRY_COUNT_LIMIT":  "1",
				"RETRY_INTERVAL_SEC": "5",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", imageChartPath, chartPath)
			assert.NotNil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_25_helm_default_retry_successful
		Assess("helm default retry successful", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify secrets provider fails
			pod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), SPHelmLabelSelector)
			assert.Nil(t, err)

			// verify logs contain CSPFK010E & 1 attempt to retry authenticate
			logs, err := GetPodLogs(cfg.Client(), pod.Name, SecretsProviderNamespace(), LogsContainer)
			assert.Nil(t, err)

			defaultRetryIntervalSec := 1
			defaultRetryCountLimit := 5

			fmt.Printf("Expecting Secrets Provider retry configurations to take defaults RETRY_INTERVAL_SEC %d and RETRY_COUNT_LIMIT %d\n", defaultRetryIntervalSec, defaultRetryCountLimit)

			assert.Contains(t, logs.String(), "CSPFK010E Failed to authenticate")
			assert.Contains(t, logs.String(), fmt.Sprintf("CSPFK010I Updating Kubernetes Secrets: 1 retries out of %d", defaultRetryCountLimit))

			// verify retry intervals are correct (roughly 5 seconds apart)
			retryIntervalMin := float64(float64(defaultRetryIntervalSec) / 100 * 80)
			retryIntervalMax := float64(float64(defaultRetryIntervalSec) / 100 * 160)

			t1, err := GetTimestamp(logs.String(), "CSPFK010E Failed to authenticate")
			assert.Nil(t, err)

			t2, err := GetTimestamp(logs.String(), fmt.Sprintf("CSPFK010I Updating Kubernetes Secrets: 1 retries out of %d", defaultRetryCountLimit))
			assert.Nil(t, err)

			duration := t2.Sub(t1).Seconds()

			assert.LessOrEqual(t, duration, retryIntervalMax)
			assert.GreaterOrEqual(t, duration, retryIntervalMin)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up job
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestHelmOverrideDefaultRetrySuccessful(t *testing.T) {
	f := features.New("helm override default retry successful").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// set up job
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{
				"LABELS":    "app: test-helm",
				"LOG_LEVEL": "debug",
				// parameter that will force failure
				"CONJUR_AUTHN_URL":   ConjurAuthnUrl() + "xyz",
				"RETRY_COUNT_LIMIT":  "2",
				"RETRY_INTERVAL_SEC": "5",
			})
			assert.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			assert.NotNil(t, err)

			return ctx
		}).
		// Replaces TEST_ID_26_helm_override_default_retry_successful
		Assess("helm override default retry successful", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// verify secrets provider fails
			pod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), SPHelmLabelSelector)
			assert.Nil(t, err)

			// verify logs contain CSPFK010E & 2 attempts to retry authenticate
			logs, err := GetPodLogs(cfg.Client(), pod.Name, SecretsProviderNamespace(), LogsContainer)
			assert.Nil(t, err)

			retryIntervalSec := 5
			retryCountLimit := 2

			fmt.Printf("Expecting Secrets Provider retry configurations to take their defaults of RETRY_INTERVAL_SEC of %d and RETRY_COUNT_LIMIT of %d", retryIntervalSec, retryCountLimit)

			assert.Contains(t, logs.String(), "CSPFK010E Failed to authenticate")
			assert.Contains(t, logs.String(), fmt.Sprintf("CSPFK010I Updating Kubernetes Secrets: 1 retries out of %d", retryCountLimit))

			// verify retry intervals are correct (roughly 5 seconds apart)
			retryIntervalMin := float64(float64(retryIntervalSec) / 100 * 80)
			retryIntervalMax := float64(float64(retryIntervalSec) / 100 * 160)

			t1, err := GetTimestamp(logs.String(), "CSPFK010E Failed to authenticate")
			assert.Nil(t, err)

			t2, err := GetTimestamp(logs.String(), fmt.Sprintf("CSPFK010I Updating Kubernetes Secrets: 1 retries out of %d", retryCountLimit))
			assert.Nil(t, err)

			duration := t2.Sub(t1).Seconds()

			assert.LessOrEqual(t, duration, retryIntervalMax)
			assert.GreaterOrEqual(t, duration, retryIntervalMin)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// clean up job
			err := RemoveJobWithHelm(cfg, "secrets-provider")
			assert.Nil(t, err)

			return ctx
		})

	testenv.Test(t, f.Feature())
}
