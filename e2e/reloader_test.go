//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
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

// TestReloaderTriggersApplicationRestart tests that the Reloader subchart
// successfully detects secret changes and triggers application restarts.
// This is only relevant for standalone/Job mode where secrets-provider
// creates K8s Secrets that applications consume.
func TestReloaderTriggersApplicationRestart(t *testing.T) {
	f := features.New("reloader triggers application restart on secret change").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
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
		Assess("application restarts when secret changes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
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

			// Manually update the K8s secret until standalone mode is supported in tests
			err = UpdateK8sSecretKeyValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "updated-secret-value", 30*time.Second)
			require.Nil(t, err)

			// Verify K8s secret was updated
			err = WaitForK8sSecretValue(cfg.Client(), SecretsProviderNamespace(), "test-k8s-secret", "secret", "updated-secret-value", 30*time.Second)
			require.Nil(t, err)

			// Wait for reloader to detect the change and restart the pod
			// The pod should have a different UID after restart
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
			// Clean up resources
			_ = RemoveJobWithHelm(cfg, "secrets-provider")

			// Delete test app deployment
			deployment, _ := GetDeployment(cfg.Client(), "test-env")
			if deployment != nil {
				_ = DeleteDeployment(cfg.Client(), SecretsProviderNamespace(), deployment)
			}

			return ctx
		})

	testenv.Test(t, f.Feature())
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
							Image:   "centos:7",
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

func UpdateK8sSecretKeyValue(client klient.Client, namespace string, secretName string, key string, value string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout updating secret %s/%s key %s: %v", namespace, secretName, key, lastErr)
		case <-ticker.C:
			var secret corev1.Secret
			err := client.Resources(namespace).Get(ctx, secretName, namespace, &secret)
			if err != nil {
				lastErr = err
				continue
			}

			if secret.Data == nil {
				secret.Data = map[string][]byte{}
			}
			secret.Data[key] = []byte(value)

			err = client.Resources(namespace).Update(ctx, &secret)
			if err != nil {
				lastErr = err
				continue
			}
			return nil
		}
	}
}
