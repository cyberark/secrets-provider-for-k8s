//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// TestReloaderTriggersApplicationRestartInApplicationMode verifies the full end-to-end
// secret rotation flow in application mode (Job-based deployment) with Reloader enabled.
//
// Scenario: A secrets-provider Job runs to provision initial secrets, after which a workload
// starts consuming them. When the upstream Conjur secret is updated, a second Job run
// (simulating a CronJob or external trigger) syncs the change to the Kubernetes Secret.
// Reloader detects that Kubernetes Secret update and triggers a rolling restart of the
// workload so it picks up the new value automatically.
//
// This test verifies:
//   - The secrets-provider Job correctly populates a Kubernetes Secret from Conjur on first run
//   - A second Job run syncs an updated Conjur secret value to the same Kubernetes Secret
//   - Reloader detects the Kubernetes Secret change and restarts the application pod
//   - The restarted pod reads the updated secret value
//
// Why this matters: In application mode the provider does not run continuously — secret
// updates require re-running the Job. Reloader bridges the gap between that external trigger
// and the workload automatically rolling over to the new secret value without redeployment.
func TestReloaderTriggersApplicationRestartInApplicationMode(t *testing.T) {
	f := features.New("reloader triggers application restart on secret change").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Register cleanup to restore secret even if Setup or Assess fails before Teardown runs
			t.Cleanup(func() {
				_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "supersecret")
			})

			// Set initial Conjur secret value
			err := SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "initial-secret-value")
			require.Nil(t, err)

			// Ensure the K8s secret exists with the expected conjur-map
			err = EnsureTestK8sSecretForReloader(cfg.Client())
			require.Nil(t, err)

			// Deploy secrets-provider Job with Reloader enabled
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{
				"RELOADER_ENABLED":             "true",
				"RELOADER_ENABLE_HA":           "false",
				"RELOADER_DEPLOYMENT_REPLICAS": "1",
			})
			require.Nil(t, err)

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			require.Nil(t, err)

			// Wait for secrets-provider job to complete
			job, err := GetJob(cfg.Client(), "secrets-provider")
			require.Nil(t, err)
			err = WaitJobCompleted(cfg.Client(), job)
			require.Nil(t, err)

			// Verify initial secret value was set
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "initial-secret-value", 30*time.Second)
			require.Nil(t, err)

			// Clean up any existing test-env deployment to avoid AlreadyExists collision
			if existingDeployment, _ := GetDeployment(cfg.Client(), "test-env"); existingDeployment != nil {
				_ = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), existingDeployment)
			}

			// Deploy test application with reloader annotation
			err = DeployTestAppWithReloaderAnnotation(cfg.Client(), "")
			require.Nil(t, err)

			// Wait for test app pod to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app=test-env", 60*time.Second)
			require.Nil(t, err)

			// Verify reloader deployment is running
			reloaderDeployment, err := GetDeployment(cfg.Client(), "secrets-provider-reloader")
			require.Nil(t, err)
			require.NotNil(t, reloaderDeployment)

			// Wait for reloader to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app.kubernetes.io/name=reloader", 60*time.Second)
			require.Nil(t, err)

			// Wait for the test-env deployment to stabilize: exactly one Running+Ready pod.
			// The reloader may trigger a rollout in response to the initial K8s secret
			// population by the secrets-provider Job. If that rollout is still in progress
			// when the Assess phase begins, WaitForPodRestart would immediately find the
			// rollout pod (created before the K8s secret is updated to "updated-secret-value"),
			// causing the subsequent secret-value check to fail.
			err = WaitForSingleReadyPod(cfg.Client(), SecretsProviderNamespace(), "app=test-env", 120*time.Second)
			require.Nil(t, err)

			return ctx
		}).
		Assess("application restarts when secret changes via secrets-provider job run", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Get initial pod to track restart
			initialPod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), "app=test-env")
			require.Nil(t, err)
			initialPodUID := initialPod.UID
			initialPodName := initialPod.Name

			fmt.Printf("Initial test app pod: %s (UID: %s)\n", initialPodName, initialPodUID)

			// Verify application has initial secret value
			err = WaitForSecretValue(cfg.Client(), "app=test-env", "test-app", "TEST_SECRET", "initial-secret-value", 30*time.Second)
			require.Nil(t, err)

			// Update the Conjur secret value
			err = SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "updated-secret-value")
			require.Nil(t, err)

			// Run the Secrets Provider Job again to sync the updated secret to K8s
			// This simulates the job running again (via CronJob or external trigger)
			fmt.Println("Running Secrets Provider Job again to sync updated secrets...")
			chartPath, err := FillHelmChart(cfg.Client(), "", map[string]string{
				"RELOADER_ENABLED":             "true",
				"RELOADER_ENABLE_HA":           "false",
				"RELOADER_DEPLOYMENT_REPLICAS": "1",
			})
			require.Nil(t, err, "Failed to generate Helm chart for second job run")

			err = DeploySecretsProviderJobWithHelm(cfg, "", chartPath)
			require.Nil(t, err, "Failed to deploy Secrets Provider Job for second run")

			// Wait for the job to complete
			job, err := GetJob(cfg.Client(), "secrets-provider")
			require.Nil(t, err)
			err = WaitJobCompleted(cfg.Client(), job)
			require.Nil(t, err, "Secrets Provider Job did not complete")

			// Verify K8s secret was updated by the job
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "updated-secret-value", 30*time.Second)
			require.Nil(t, err, "K8s secret was not updated by Secrets Provider Job")

			// Wait for reloader to detect the K8s secret change and restart the pod
			// The pod should have a different UID after restart
			fmt.Println("Waiting for reloader to restart the application pod...")
			err = WaitForPodRestart(cfg.Client(), SecretsProviderNamespace(), "app=test-env", initialPodUID, 120*time.Second)
			assert.Nil(t, err, "Reloader should have triggered a pod restart after K8s secret change")

			// Get the new pod
			newPod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), "app=test-env")
			require.Nil(t, err)
			fmt.Printf("New test app pod after restart: %s (UID: %s)\n", newPod.Name, newPod.UID)

			// Verify the new pod has the updated secret value
			err = WaitForSecretValue(cfg.Client(), "app=test-env", "test-app", "TEST_SECRET", "updated-secret-value", 120*time.Second)
			assert.Nil(t, err, "Application should have the updated secret value after reloader triggered restart")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Restore Conjur secrets used by this test so subsequent tests see expected values
			_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "supersecret")

			// Clean up resources
			_ = RemoveHelmRelease(cfg, "secrets-provider")

			// Delete test app deployment
			deployment, _ := GetDeployment(cfg.Client(), "test-env")
			if deployment != nil {
				_ = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			}

			return ctx
		})

	testenv.Test(t, f.Feature())
}

// reloaderTestAppImage returns the container image used for test app pods in reloader E2E tests.
// The container only needs to sleep and expose an env var — any maintained minimal image works.
// Override with RELOADER_TEST_APP_IMAGE to use a registry-local mirror, if needed.
func reloaderTestAppImage() string {
	if img := os.Getenv("RELOADER_TEST_APP_IMAGE"); img != "" {
		return img
	}
	return "busybox:stable"
}

// DeployTestAppWithReloaderAnnotation deploys a test application with the
// conjur.org/automatic-restart annotation that tells Reloader to watch for changes
func DeployTestAppWithReloaderAnnotation(client klient.Client, id string) error {
	var replicas int32 = 1
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": id + "test-env",
			},
			Name:      id + "test-env",
			Namespace: SecretsProviderNamespace(),
			Annotations: map[string]string{
				// conjur.org/automatic-restart is an alias for secret.reloader.stakater.com/auto,
				// configured via reloader.reloader.custom_annotations.secret_auto in values.yaml.
				// It tells Reloader to restart this deployment when any referenced Secret changes.
				"conjur.org/automatic-restart": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": id + "test-env",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": id + "test-env",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "secrets-provider-service-account",
					Containers: []corev1.Container{
						{
							Image:   reloaderTestAppImage(),
							Name:    id + "test-app",
							Command: []string{"sleep"},
							Args:    []string{"infinity"},
							Env: []corev1.EnvVar{
								{
									Name: id + "TEST_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: id + "test-k8s-secret",
											},
											Key: "secret",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	err := client.Resources().Create(context.TODO(), &deployment)
	if err != nil {
		return err
	}

	// Wait for deployment to be ready
	err = WaitResourceScaled(client, &deployment, replicas)
	if err != nil {
		return err
	}

	return nil
}

func EnsureTestK8sSecretForReloader(client klient.Client) error {
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-k8s-secret",
			Namespace: SecretsProviderNamespace(),
			Labels: map[string]string{
				"conjur.org/managed-by-provider": "true",
			},
		},
		StringData: map[string]string{
			"conjur-map": "secret: data/secrets/test_secret",
		},
		Type: "Opaque",
	}

	if err := client.Resources().Create(context.TODO(), &secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			var existingSecret corev1.Secret
			getErr := client.Resources().Get(context.TODO(), secret.Name, secret.Namespace, &existingSecret)
			if getErr != nil {
				return getErr
			}

			if existingSecret.Labels == nil {
				existingSecret.Labels = map[string]string{}
			}
			existingSecret.Labels["conjur.org/managed-by-provider"] = "true"

			if existingSecret.Data == nil {
				existingSecret.Data = map[string][]byte{}
			}
			existingSecret.Data["conjur-map"] = []byte("secret: data/secrets/test_secret")
			existingSecret.Type = "Opaque"

			return client.Resources().Update(context.TODO(), &existingSecret)
		}
		return err
	}

	return nil
}

// TestReloaderTriggersApplicationRestartInStandaloneModeSingleNamespace verifies the full
// end-to-end secret rotation flow in standalone mode (Deployment-based) with Reloader in
// a single namespace.
//
// Scenario: The secrets-provider runs continuously as a Deployment rather than a one-shot
// Job. It discovers Kubernetes Secrets by label and refreshes their values from Conjur on
// a short interval. When a Conjur secret changes, the provider automatically syncs the new
// value to the Kubernetes Secret on its next cycle — no external trigger needed. Reloader
// then detects the Kubernetes Secret update and restarts the workload.
//
// This test verifies:
//   - The secrets-provider Deployment (not Job) starts and populates secrets discovered by label
//   - When a Conjur secret is updated, the provider's periodic refresh syncs it to the
//     Kubernetes Secret without any manual intervention
//   - Reloader detects the Kubernetes Secret update and triggers a pod restart
//   - The restarted pod reads the updated secret value
//
// Why this matters: Standalone mode is designed for environments where secrets must stay
// current without re-deploying the provider. This test validates the fully automated
// rotation loop — Conjur change → provider sync → Reloader restart — with no human action.
func TestReloaderTriggersApplicationRestartInStandaloneModeSingleNamespace(t *testing.T) {
	f := features.New("reloader triggers application restart on secret change in standalone mode (single namespace)").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Register cleanup to restore secret even if Setup or Assess fails before Teardown runs
			t.Cleanup(func() {
				_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "supersecret")
			})

			// Set initial Conjur secret value
			err := SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "initial-secret-value")
			require.Nil(t, err)

			// Create the K8s secret with label for standalone mode discovery
			err = EnsureTestK8sSecretForReloaderWithLabel(cfg.Client())
			require.Nil(t, err)

			// Deploy secrets-provider in Standalone mode with Reloader enabled
			// In standalone mode, the provider runs as a Deployment and periodically refreshes secrets
			chartPath, err := FillHelmChartForMode(cfg.Client(), "", DeployModeStandalone, map[string]string{
				"RELOADER_ENABLED":             "true",
				"RELOADER_ENABLE_HA":           "false",
				"RELOADER_DEPLOYMENT_REPLICAS": "1",
				"NAMESPACE_ALLOWLIST":          "", // Empty = use current namespace only
				"SECRETS_REFRESH_ENABLED":      "true",
				"SECRETS_REFRESH_INTERVAL":     "10s", // Short interval for testing
			})
			require.Nil(t, err)

			err = DeploySecretsProviderDeploymentWithHelm(cfg, "", chartPath)
			require.Nil(t, err)

			// Verify secrets-provider Deployment is running (not a Job)
			deployment, err := GetDeployment(cfg.Client(), "secrets-provider")
			require.Nil(t, err)
			require.NotNil(t, deployment)

			// Wait for secrets-provider pod to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app=secrets-provider", 60*time.Second)
			require.Nil(t, err)

			// Wait for initial secret value to be populated by the provider
			// In standalone mode, the provider refreshes periodically, so we wait for the label-based secret to be updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "initial-secret-value", 30*time.Second)
			require.Nil(t, err)

			// Clean up any existing test-env deployment to avoid AlreadyExists collision
			if existingDeployment, _ := GetDeployment(cfg.Client(), "test-env"); existingDeployment != nil {
				_ = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), existingDeployment)
			}

			// Deploy test application with reloader annotation
			err = DeployTestAppWithReloaderAnnotation(cfg.Client(), "")
			require.Nil(t, err)

			// Wait for test app pod to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app=test-env", 60*time.Second)
			require.Nil(t, err)

			// Verify reloader deployment is running
			reloaderDeployment, err := GetDeployment(cfg.Client(), "secrets-provider-reloader")
			require.Nil(t, err)
			require.NotNil(t, reloaderDeployment)

			// Wait for reloader to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app.kubernetes.io/name=reloader", 60*time.Second)
			require.Nil(t, err)

			// Wait for the test-env deployment to stabilize: exactly one Running+Ready pod
			err = WaitForSingleReadyPod(cfg.Client(), SecretsProviderNamespace(), "app=test-env", 120*time.Second)
			require.Nil(t, err)

			return ctx
		}).
		Assess("application restarts when secret changes in Conjur", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Get initial pod to track restart
			initialPod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), "app=test-env")
			require.Nil(t, err)
			initialPodUID := initialPod.UID
			initialPodName := initialPod.Name

			fmt.Printf("Initial test app pod: %s (UID: %s)\n", initialPodName, initialPodUID)

			// Verify application has initial secret value
			err = WaitForSecretValue(cfg.Client(), "app=test-env", "test-app", "TEST_SECRET", "initial-secret-value", 30*time.Second)
			require.Nil(t, err)

			// Update the Conjur secret
			err = SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "updated-secret-value")
			require.Nil(t, err)

			// In standalone mode, the secrets-provider Deployment will automatically detected the change
			// and update the K8s secret as part of its periodic refresh cycle (within 10 seconds)
			fmt.Println("Waiting for standalone secrets-provider to detect and sync secret change...")
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "updated-secret-value", 30*time.Second)
			require.Nil(t, err, "Standalone secrets-provider should have automatically updated K8s secret")

			// Wait for reloader to detect the K8s secret change and restart the pod
			fmt.Println("Waiting for reloader to restart the application pod...")
			err = WaitForPodRestart(cfg.Client(), SecretsProviderNamespace(), "app=test-env", initialPodUID, 120*time.Second)
			assert.Nil(t, err, "Reloader should have triggered a pod restart after secret change")

			// Get the new pod
			newPod, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), "app=test-env")
			require.Nil(t, err)
			fmt.Printf("New test app pod after restart: %s (UID: %s)\n", newPod.Name, newPod.UID)

			// Verify the new pod has the updated secret value
			err = WaitForSecretValue(cfg.Client(), "app=test-env", "test-app", "TEST_SECRET", "updated-secret-value", 120*time.Second)
			assert.Nil(t, err, "Application should have the updated secret value after reloader triggered restart")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Restore Conjur secrets used by this test so subsequent tests see expected values
			_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret", "supersecret")

			// Clean up resources
			_ = RemoveHelmRelease(cfg, "secrets-provider")

			// Delete test app deployment
			deployment, _ := GetDeployment(cfg.Client(), "test-env")
			if deployment != nil {
				_ = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			}

			return ctx
		})

	testenv.Test(t, f.Feature())
}

// TestReloaderTriggersApplicationRestartInStandaloneModeMultiNamespace verifies that
// standalone mode works correctly across multiple Kubernetes namespaces, and that Reloader
// handles simultaneous secret updates in those namespaces independently.
//
// Scenario: Two workloads in separate namespaces each consume a different Conjur secret.
// The secrets-provider Deployment is configured with a namespace allowlist so it manages
// Kubernetes Secrets in both namespaces via a ClusterRole. Both Conjur secrets are updated
// at the same time, and the provider syncs both changes on its next refresh cycle. Reloader
// detects the update in each namespace and independently restarts each workload.
//
// This test verifies:
//   - The secrets-provider Deployment manages secrets across multiple namespaces via
//     the namespaceAllowlist configuration
//   - RBAC (ClusterRole/ClusterRoleBinding) is correctly configured for cross-namespace access
//   - Reloader runs with watchGlobally enabled (required for multi-namespace allowlist + autoReload per chart schema)
//   - Simultaneous Conjur secret updates in different namespaces are both synced by the provider
//   - Reloader restarts workloads in each namespace independently when their secrets change
//   - Each restarted workload reads its own updated secret value
//
// Why this matters: Production environments typically span many namespaces. This test
// validates that multi-namespace configuration, RBAC, and parallel secret refresh all work
// together, and that Reloader reacts to each namespace's changes without cross-contamination.
func TestReloaderTriggersApplicationRestartInStandaloneModeMultiNamespace(t *testing.T) {
	f := features.New("reloader works with standalone mode across multiple namespaces and multiple secret updates").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create a second test namespace for multi-namespace testing
			secondNamespace := SecretsProviderNamespace() + "-2"
			err := EnsureNamespace(cfg.Client(), secondNamespace)
			require.Nil(t, err)

			// Set initial Conjur secret values
			err = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_1", "initial-value-1")
			require.Nil(t, err)
			err = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_2", "initial-value-2")
			require.Nil(t, err)

			// Register cleanup to restore secrets even if Setup fails (Teardown only runs after Assess)
			t.Cleanup(func() {
				_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_1", "some-secret")
				_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_2", "some-secret")
			})

			// Create K8s secrets in both namespaces with labels for discovery
			err = EnsureTestK8sSecretWithLabel(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-1", "secret", "data/secrets/test_secret_1")
			require.Nil(t, err)
			err = EnsureTestK8sSecretWithLabel(cfg.Client(), secondNamespace, "test-k8s-secret-2", "secret", "data/secrets/test_secret_2")
			require.Nil(t, err)

			namespaceAllowlist := SecretsProviderNamespace() + "," + secondNamespace

			// Deploy secrets-provider in Standalone mode with multi-namespace support
			chartPath, err := FillHelmChartForMode(cfg.Client(), "", DeployModeStandalone, map[string]string{
				"RELOADER_ENABLED":             "true",
				"RELOADER_WATCH_GLOBALLY":      "true",
				"RELOADER_ENABLE_HA":           "false",
				"RELOADER_DEPLOYMENT_REPLICAS": "1",
				"NAMESPACE_ALLOWLIST":          namespaceAllowlist,
				"SECRETS_REFRESH_ENABLED":      "true",
				"SECRETS_REFRESH_INTERVAL":     "10s",
			})
			require.Nil(t, err)

			err = DeploySecretsProviderDeploymentWithHelm(cfg, "", chartPath)
			require.Nil(t, err)

			// Verify secrets-provider Deployment is running with ClusterRole (for multi-namespace access)
			deployment, err := GetDeployment(cfg.Client(), "secrets-provider")
			require.Nil(t, err)
			require.NotNil(t, deployment)

			// Wait for secrets-provider pod to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app=secrets-provider", 60*time.Second)
			require.Nil(t, err)

			// Wait for initial secret values to be populated in both namespaces
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-1", "secret", "initial-value-1", 30*time.Second)
			require.Nil(t, err)
			err = WaitForK8sSecretValue(cfg.Client(), secondNamespace, "test-k8s-secret-2", "secret", "initial-value-2", 30*time.Second)
			require.Nil(t, err)

			// Clean up any existing test apps to avoid conflicts
			if existingDeployment, _ := GetDeployment(cfg.Client(), "test-env-1"); existingDeployment != nil {
				_ = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), existingDeployment)
			}
			if existingDeployment, _ := GetDeployment(cfg.Client(), "test-env-2"); existingDeployment != nil {
				_ = DeleteDeployment(cfg.Client(), secondNamespace, existingDeployment)
			}

			// Deploy test applications in both namespaces
			err = DeployTestAppWithReloaderAnnotationInNamespace(cfg.Client(), "", SecretsProviderNamespace(), "test-k8s-secret-1")
			require.Nil(t, err)
			err = DeployTestAppWithReloaderAnnotationInNamespace(cfg.Client(), "", secondNamespace, "test-k8s-secret-2")
			require.Nil(t, err)

			// Wait for both test apps to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app=test-env", 60*time.Second)
			require.Nil(t, err)
			err = WaitForPodReady(cfg.Client(), secondNamespace, "app=test-env", 60*time.Second)
			require.Nil(t, err)

			// Wait for reloader to be ready
			err = WaitForPodReady(cfg.Client(), SecretsProviderNamespace(), "app.kubernetes.io/name=reloader", 60*time.Second)
			require.Nil(t, err)

			// Wait for test apps to stabilize
			err = WaitForSingleReadyPod(cfg.Client(), SecretsProviderNamespace(), "app=test-env", 120*time.Second)
			require.Nil(t, err)
			err = WaitForSingleReadyPod(cfg.Client(), secondNamespace, "app=test-env", 120*time.Second)
			require.Nil(t, err)

			return ctx
		}).
		Assess("multiple secrets are updated simultaneously and applications restart", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			secondNamespace := SecretsProviderNamespace() + "-2"

			// Get initial pods to track restarts
			initialPod1, err := FetchPodWithLabelSelector(cfg.Client(), SecretsProviderNamespace(), "app=test-env")
			require.Nil(t, err)
			initialPod2, err := FetchPodWithLabelSelector(cfg.Client(), secondNamespace, "app=test-env")
			require.Nil(t, err)

			fmt.Printf("Initial pods: ns1=%s, ns2=%s\n", initialPod1.Name, initialPod2.Name)

			// Verify applications have initial secret values
			err = WaitForSecretValueInNamespace(cfg.Client(), SecretsProviderNamespace(), "app=test-env", "test-app", "TEST_SECRET", "initial-value-1", 30*time.Second)
			require.Nil(t, err)
			err = WaitForSecretValueInNamespace(cfg.Client(), secondNamespace, "app=test-env", "test-app", "TEST_SECRET", "initial-value-2", 30*time.Second)
			require.Nil(t, err)

			// Update both Conjur secrets simultaneously
			err = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_1", "updated-value-1")
			require.Nil(t, err)
			err = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_2", "updated-value-2")
			require.Nil(t, err)

			// Wait for standalone provider to sync both secrets in parallel
			fmt.Println("Waiting for standalone provider to sync multiple secrets...")
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret-1", "secret", "updated-value-1", 30*time.Second)
			require.Nil(t, err)
			err = WaitForK8sSecretValue(cfg.Client(), secondNamespace, "test-k8s-secret-2", "secret", "updated-value-2", 30*time.Second)
			require.Nil(t, err)

			// Wait for reloader to restart both applications
			fmt.Println("Waiting for reloader to restart both applications...")
			err = WaitForPodRestart(cfg.Client(), SecretsProviderNamespace(), "app=test-env", initialPod1.UID, 120*time.Second)
			assert.Nil(t, err, "Reloader should have triggered restart in namespace 1")

			err = WaitForPodRestart(cfg.Client(), secondNamespace, "app=test-env", initialPod2.UID, 120*time.Second)
			assert.Nil(t, err, "Reloader should have triggered restart in namespace 2")

			// Verify both applications received updated secrets
			err = WaitForSecretValueInNamespace(cfg.Client(), SecretsProviderNamespace(), "app=test-env", "test-app", "TEST_SECRET", "updated-value-1", 120*time.Second)
			assert.Nil(t, err, "Application in namespace 1 should have updated secret")

			// Verify ns2 pod also received its updated secret value
			newPod2, err := FetchPodWithLabelSelector(cfg.Client(), secondNamespace, "app=test-env")
			require.Nil(t, err)
			fmt.Printf("New ns2 pod after restart: %s (UID: %s)\n", newPod2.Name, newPod2.UID)
			err = WaitForSecretValueInNamespace(cfg.Client(), secondNamespace, "app=test-env", "test-app", "TEST_SECRET", "updated-value-2", 120*time.Second)
			assert.Nil(t, err, "Application in namespace 2 should have updated secret")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			secondNamespace := SecretsProviderNamespace() + "-2"

			// Restore Conjur secrets used by this test so subsequent tests see expected values
			_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_1", "some-secret")
			_ = SetConjurSecret(cfg.Client(), "data/secrets/test_secret_2", "some-secret")

			// Clean up resources
			_ = RemoveHelmRelease(cfg, "secrets-provider")

			// Delete test apps
			if deployment, _ := GetDeployment(cfg.Client(), "test-env"); deployment != nil {
				_ = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			}
			if deployment, _ := GetDeploymentInNamespace(cfg.Client(), "test-env", secondNamespace); deployment != nil {
				_ = DeleteDeployment(cfg.Client(), secondNamespace, deployment)
			}

			// Clean up second namespace
			_ = DeleteNamespaceIgnoringNotFound(cfg.Client(), secondNamespace)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

// EnsureTestK8sSecretForReloaderWithLabel ensures a test K8s secret exists with label for standalone mode discovery
func EnsureTestK8sSecretForReloaderWithLabel(client klient.Client) error {
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-k8s-secret",
			Namespace: SecretsProviderNamespace(),
			Labels: map[string]string{
				"conjur.org/managed-by-provider": "true",
				// Label for standalone mode discovery (in addition to explicit config)
				"conjur.org/template-engine": "true",
			},
		},
		StringData: map[string]string{
			"conjur-map": "secret: data/secrets/test_secret",
		},
		Type: "Opaque",
	}

	if err := client.Resources().Create(context.TODO(), &secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			var existingSecret corev1.Secret
			getErr := client.Resources().Get(context.TODO(), secret.Name, secret.Namespace, &existingSecret)
			if getErr != nil {
				return getErr
			}

			if existingSecret.Labels == nil {
				existingSecret.Labels = map[string]string{}
			}
			existingSecret.Labels["conjur.org/managed-by-provider"] = "true"
			existingSecret.Labels["conjur.org/template-engine"] = "true"

			if existingSecret.Data == nil {
				existingSecret.Data = map[string][]byte{}
			}
			existingSecret.Data["conjur-map"] = []byte("secret: data/secrets/test_secret")
			existingSecret.Type = "Opaque"

			return client.Resources().Update(context.TODO(), &existingSecret)
		}
		return err
	}

	return nil
}

// EnsureTestK8sSecretWithLabel ensures a test K8s secret exists in a specific namespace with label and conjur-map
func EnsureTestK8sSecretWithLabel(client klient.Client, namespace string, secretName string, valueKey string, conjurPath string) error {
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"conjur.org/managed-by-provider": "true",
				"conjur.org/template-engine":     "true",
			},
		},
		StringData: map[string]string{
			"conjur-map": fmt.Sprintf("%s: %s", valueKey, conjurPath),
		},
		Type: "Opaque",
	}

	if err := client.Resources().Create(context.TODO(), &secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			var existingSecret corev1.Secret
			getErr := client.Resources().Get(context.TODO(), secretName, namespace, &existingSecret)
			if getErr != nil {
				return getErr
			}

			if existingSecret.Labels == nil {
				existingSecret.Labels = map[string]string{}
			}
			existingSecret.Labels["conjur.org/managed-by-provider"] = "true"
			existingSecret.Labels["conjur.org/template-engine"] = "true"

			if existingSecret.Data == nil {
				existingSecret.Data = map[string][]byte{}
			}
			existingSecret.Data["conjur-map"] = []byte(fmt.Sprintf("%s: %s", valueKey, conjurPath))
			existingSecret.Type = "Opaque"

			return client.Resources().Update(context.TODO(), &existingSecret)
		}
		return err
	}

	return nil
}

// DeployTestAppWithReloaderAnnotationInNamespace deploys a test application in a specific namespace
func DeployTestAppWithReloaderAnnotationInNamespace(client klient.Client, id string, namespace string, secretName string) error {
	var replicas int32 = 1
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": id + "test-env",
			},
			Name:      id + "test-env",
			Namespace: namespace,
			Annotations: map[string]string{
				"conjur.org/automatic-restart": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": id + "test-env",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": id + "test-env",
					},
				},
				Spec: corev1.PodSpec{
					// Use default SA — this pod only reads a K8s secret via env var and doesn't
					// need Conjur auth, so it works in any namespace without special setup.
					Containers: []corev1.Container{
						{
							Image:   reloaderTestAppImage(),
							Name:    id + "test-app",
							Command: []string{"sleep"},
							Args:    []string{"infinity"},
							Env: []corev1.EnvVar{
								{
									Name: id + "TEST_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: id + secretName,
											},
											Key: "secret",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	err := client.Resources().Create(context.TODO(), &deployment)
	if err != nil {
		return err
	}

	// Wait for deployment to be ready
	err = WaitResourceScaled(client, &deployment, replicas)
	if err != nil {
		return err
	}

	return nil
}
