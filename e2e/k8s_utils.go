//go:build e2e
// +build e2e

// Package e2e contains end-to-end tests for the Secrets Provider.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

// ReloadWithTemplate redeploys the test environment using the specified template and waits for the pod to be ready.
func ReloadWithTemplate(client klient.Client, template string) error {
	fmt.Println("Reloading test environment with template " + template)
	os.Setenv("TEMPLATE_OVERRIDE", template)
	cmd := exec.Command("../deploy/redeploy.sh")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error running redeploy.sh. Stdout:\n" + string(out))
		return err
	}

	fmt.Println("Waiting for secrets provider pod to be ready...")
	err = WaitForPodReady(client, SecretsProviderNamespace(), SPLabelSelector, 2*time.Minute)
	if err != nil {
		fmt.Println("Error waiting for Secrets Provider pod to be 'Ready' after redeploy. Stdout:\n" + string(out))
		return err
	}

	return nil
}

// WaitForPodReady polls until a pod matching the label selector is ready. This avoids race
// conditions where a specific pod may be replaced (e.g. during rollout) before becoming ready.
func WaitForPodReady(client klient.Client, namespace, labelSelector string, timeout time.Duration) error {
	condition := func(ctx context.Context) (bool, error) {
		pods, err := GetPods(client, namespace, labelSelector)
		if err != nil {
			fmt.Printf("WaitForPodReady: error listing pods (namespace=%s, selector=%s): %v\n", namespace, labelSelector, err)
			return false, nil
		}
		if len(pods.Items) == 0 {
			return false, nil
		}
		resources := client.Resources(namespace)
		for i := range pods.Items {
			pod := &pods.Items[i]
			var current corev1.Pod
			if err := resources.Get(ctx, pod.Name, pod.Namespace, &current); err != nil {
				continue // pod may have been replaced, skip and retry
			}
			for _, cond := range current.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return wait.For(condition, wait.WithTimeout(timeout), wait.WithInterval(2*time.Second))
}

// WaitForPodRestart waits for a pod matching the label selector to restart (have a different UID than originalPodUID)
func WaitForPodRestart(client klient.Client, namespace, labelSelector string, originalPodUID types.UID, timeout time.Duration) error {
	condition := func(ctx context.Context) (bool, error) {
		pods, err := GetPods(client, namespace, labelSelector)
		if err != nil {
			fmt.Printf("WaitForPodRestart: error listing pods (namespace=%s, selector=%s): %v\n", namespace, labelSelector, err)
			return false, nil
		}
		// Check if any pod has a different UID and is ready
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.UID != originalPodUID {
				// Found a new pod, check if it's ready
				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						fmt.Printf("Pod restarted: old UID %s, new UID %s\n", originalPodUID, pod.UID)
						return true, nil
					}
				}
			}
		}
		return false, nil
	}
	return wait.For(condition, wait.WithTimeout(timeout), wait.WithInterval(2*time.Second))
}

// WaitForClusterConnectivity polls until the Kubernetes API server is reachable.
// This is critical before running e2e tests, especially when cluster connectivity may be transient.
func WaitForClusterConnectivity(client klient.Client, timeout time.Duration) error {
	condition := func(ctx context.Context) (bool, error) {
		// Try to list namespaces as a simple connectivity check
		err := client.Resources().List(ctx, &corev1.NamespaceList{})
		if err != nil {
			return false, nil
		}
		return true, nil
	}
	return wait.For(condition, wait.WithTimeout(timeout), wait.WithInterval(5*time.Second))
}

// ScaleDeployment scales a deployment to the specified number of replicas and waits for the operation to complete.
func ScaleDeployment(client klient.Client, deploymentName string, namespace string, labelSelector string, replicas int32) error {
	mergePatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to json marshal: %v", err)
	}

	deployment, err := GetDeployment(client, deploymentName)
	if err != nil {
		return err
	}

	err = client.Resources(namespace).Patch(context.TODO(), deployment, k8s.Patch{PatchType: types.StrategicMergePatchType, Data: mergePatch})
	if err != nil {
		return fmt.Errorf("failed to patch deployment: %v", err)
	}

	fmt.Printf("waiting for deployment to be scaled to %d replicas\n", replicas)
	if replicas > 0 {
		err := WaitResourceScaled(client, deployment, replicas)
		if err != nil {
			return err
		}
	} else {
		err := WaitResourceDeleted(client, namespace, labelSelector)
		if err != nil {
			return err
		}
	}
	fmt.Printf("deployment successfully scaled to %d replicas\n", replicas)

	return nil
}

// WaitResourceScaled waits until the deployment reaches the specified number of ready replicas.
func WaitResourceScaled(client klient.Client, deployment *appsv1.Deployment, replicas int32) error {
	err := wait.For(conditions.New(client.Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, replicas), wait.WithTimeout(time.Minute*3))
	if err != nil {
		return fmt.Errorf("failed to wait for scaling: %v", err)
	}
	return nil
}

// WaitResourceDeleted waits until all pods matching the deployment's label selector are deleted.
func WaitResourceDeleted(client klient.Client, namespace string, labelSelector string) error {
	pods, err := GetPods(client, namespace, labelSelector)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		err = wait.For(conditions.New(client.Resources()).ResourceDeleted(&pod), wait.WithTimeout(time.Minute*3))
		if err != nil {
			return fmt.Errorf("failed to wait for deletion: %v", err)
		}
	}
	return nil
}

// WaitJobCompleted waits until the given Kubernetes Job reaches a completed state.
func WaitJobCompleted(client klient.Client, job *batchv1.Job) error {
	err := wait.For(conditions.New(client.Resources()).JobCompleted(job), wait.WithTimeout(time.Minute*1))
	if err != nil {
		return fmt.Errorf("failed to wait for job completion: %v", err)
	}
	return nil
}

// GetDeployment retrieves a Deployment from the secrets provider namespace by name.
func GetDeployment(client klient.Client, deploymentName string) (*appsv1.Deployment, error) {
	var deployment appsv1.Deployment
	err := client.Resources().Get(context.TODO(), deploymentName, SecretsProviderNamespace(), &deployment)
	if err != nil {
		return &deployment, err
	}
	return &deployment, nil
}

// DeleteDeployment deletes the given deployment and waits for it to be fully removed.
func DeleteDeployment(client klient.Client, namespace string, deployment *appsv1.Deployment) error {
	err := client.Resources(namespace).Delete(context.TODO(), deployment)
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %v", err)
	}
	err = wait.For(conditions.New(client.Resources()).ResourceDeleted(deployment), wait.WithTimeout(time.Minute*3))
	if err != nil {
		return fmt.Errorf("failed to wait for deletion: %v", err)
	}
	return nil
}

// GetJob retrieves a Job from the secrets provider namespace by name.
func GetJob(client klient.Client, jobName string) (*batchv1.Job, error) {
	var job batchv1.Job
	err := client.Resources().Get(context.TODO(), jobName, SecretsProviderNamespace(), &job)
	if err != nil {
		return &job, err
	}
	return &job, nil
}

// GetSecret retrieves a Kubernetes Secret from the secrets provider namespace by name.
func GetSecret(client klient.Client, secretName string) (*corev1.Secret, error) {
	var secret corev1.Secret
	err := client.Resources().Get(context.TODO(), secretName, SecretsProviderNamespace(), &secret)
	if err != nil {
		return &secret, err
	}
	return &secret, nil
}

// DeleteSecret deletes the named Kubernetes Secret from the secrets provider namespace.
func DeleteSecret(client klient.Client, secretName string) error {
	secret, err := GetSecret(client, secretName)
	if err != nil {
		return err
	}
	err = client.Resources().Delete(context.TODO(), secret)
	if err != nil {
		return err
	}
	return nil
}

// GetServiceAccount retrieves a ServiceAccount from the secrets provider namespace by name.
func GetServiceAccount(client klient.Client, saName string) (*corev1.ServiceAccount, error) {
	var sa corev1.ServiceAccount
	err := client.Resources().Get(context.TODO(), saName, SecretsProviderNamespace(), &sa)
	if err != nil {
		return &sa, err
	}
	return &sa, nil
}

// DeleteServiceAccount deletes the named ServiceAccount from the secrets provider namespace.
func DeleteServiceAccount(client klient.Client, saName string) error {
	sa, err := GetServiceAccount(client, saName)
	if err != nil {
		return err
	}
	err = client.Resources().Delete(context.TODO(), sa)
	if err != nil {
		return err
	}
	return nil
}

// GetRoleAndBinding retrieves a Role and its RoleBinding from the secrets provider namespace.
func GetRoleAndBinding(client klient.Client, roleName string, roleBindingName string) (*rbacv1.Role, *rbacv1.RoleBinding, error) {
	var role rbacv1.Role
	var roleBinding rbacv1.RoleBinding
	err := client.Resources().Get(context.TODO(), roleName, SecretsProviderNamespace(), &role)
	if err != nil {
		return &role, &roleBinding, err
	}
	err = client.Resources().Get(context.TODO(), roleBindingName, SecretsProviderNamespace(), &roleBinding)
	if err != nil {
		return &role, &roleBinding, err
	}
	return &role, &roleBinding, nil
}

// DeleteRoleAndBinding deletes the named Role and RoleBinding from the secrets provider namespace.
func DeleteRoleAndBinding(client klient.Client, roleName string, roleBindingName string) error {
	role, roleBinding, err := GetRoleAndBinding(client, roleName, roleBindingName)
	if err != nil {
		return err
	}
	err = client.Resources().Delete(context.TODO(), role)
	if err != nil {
		return err
	}
	err = client.Resources().Delete(context.TODO(), roleBinding)
	if err != nil {
		return err
	}
	return nil
}

// GetConfigMap retrieves a ConfigMap from the secrets provider namespace by name.
func GetConfigMap(client klient.Client, configMapName string) (*corev1.ConfigMap, error) {
	var configMap corev1.ConfigMap
	err := client.Resources().Get(context.TODO(), configMapName, SecretsProviderNamespace(), &configMap)
	if err != nil {
		return &configMap, err
	}
	return &configMap, nil
}

// GetPods lists pods in a namespace filtered by a label selector.
func GetPods(client klient.Client, namespace string, labelSelector string) (corev1.PodList, error) {
	var pods corev1.PodList
	err := client.Resources(namespace).List(context.TODO(), &pods, resources.WithLabelSelector(labelSelector))
	if err != nil {
		return corev1.PodList{}, fmt.Errorf("failed to fetch pods: %v", err)
	}

	return pods, nil
}

// SetConjurSecret sets a Conjur variable value using the Conjur CLI pod.
func SetConjurSecret(client klient.Client, varID string, value string) error {
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), ConjurCLILabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "set", "-i", varID, "-v", value}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, ConjurCLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Value added") {
		return fmt.Errorf("failed to set secret")
	}
	return nil
}

// GetConjurSecret retrieves a Conjur variable value using the Conjur CLI pod.
func GetConjurSecret(client klient.Client, varID string) (string, error) {
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), ConjurCLILabelSelector)
	if err != nil {
		return "", fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "get", "-i", varID}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, ConjurCLIContainer, command, &stdout, &stderr); err != nil {
		return "", fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// RunCommandInSecretsProviderPod executes a command in the secrets provider pod, retrying on transient errors.
func RunCommandInSecretsProviderPod(client klient.Client, labelSelector string, container string, command []string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	var lastExecError error
	for attemptNumber := 1; attemptNumber <= 10; attemptNumber++ {
		if stdout != nil {
			stdout.Reset()
		}
		if stderr != nil {
			stderr.Reset()
		}

		pod, fetchError := FetchPodWithLabelSelector(client, SecretsProviderNamespace(), labelSelector)
		if fetchError != nil {
			lastExecError = fmt.Errorf("failed to fetch cli pod: %v", fetchError)
			time.Sleep(2 * time.Second)
			continue
		}

		execError := client.Resources(SecretsProviderNamespace()).ExecInPod(context.TODO(), SecretsProviderNamespace(), pod.Name, container, command, stdout, stderr)
		if execError == nil {
			return nil
		}

		lastExecError = fmt.Errorf("failed to execute command in pod %s container %s: %v; stderr: %s", pod.Name, container, execError, stderr.String())
		if !isTransientExecError(execError, stderr.String()) {
			return lastExecError
		}

		time.Sleep(2 * time.Second)
	}

	if lastExecError != nil {
		return lastExecError
	}

	return fmt.Errorf("failed to execute command in secrets provider pod for unknown reason")
}

func isTransientExecError(execError error, stderrText string) bool {
	errorText := strings.ToLower(execError.Error() + " " + stderrText)
	transientSignatures := []string{
		"unable to upgrade connection",
		"container not found",
		"pod not found",
		"i/o timeout",
		"connection refused",
		"tls handshake timeout",
	}

	for _, transientSignature := range transientSignatures {
		if strings.Contains(errorText, transientSignature) {
			return true
		}
	}

	return false
}

// FetchPodWithLabelSelector returns the first running and ready pod matching the label selector in the given namespace.
func FetchPodWithLabelSelector(client klient.Client, namespace string, labelSelector string) (corev1.Pod, error) {
	var pods corev1.PodList
	var pod corev1.Pod

	err := client.Resources(namespace).List(context.TODO(), &pods, resources.WithLabelSelector(labelSelector))
	if err != nil {
		return pod, fmt.Errorf("failed to fetch pods. %v", err)
	}

	if len(pods.Items) >= 1 {
		for _, candidatePod := range pods.Items {
			if candidatePod.Status.Phase != corev1.PodRunning {
				continue
			}
			for _, condition := range candidatePod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					return candidatePod, nil
				}
			}
		}

		return pods.Items[0], nil
	}

	return pod, fmt.Errorf("Expected exactly 1 pod to match label selector %s in namespace %s. Matching pod list: %v", labelSelector, namespace, pods.Items)
}

// WaitForK8sSecretKeys waits until all specified keys exist in a Kubernetes Secret.
func WaitForK8sSecretKeys(client klient.Client, namespace string, secretName string, keys []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastError error
	var missingKeys []string
	for {
		select {
		case <-ctx.Done():
			if len(missingKeys) > 0 {
				return fmt.Errorf("timeout waiting for secret %s/%s keys %v. Last error: %v", namespace, secretName, missingKeys, lastError)
			}
			return fmt.Errorf("timeout waiting for secret %s/%s keys. Last error: %v", namespace, secretName, lastError)
		case <-ticker.C:
			var secret corev1.Secret
			if err := client.Resources(namespace).Get(context.TODO(), secretName, namespace, &secret); err != nil {
				lastError = err
				continue
			}

			missingKeys = missingKeys[:0]
			for _, key := range keys {
				if _, exists := secret.Data[key]; !exists {
					missingKeys = append(missingKeys, key)
				}
			}
			if len(missingKeys) == 0 {
				return nil
			}
		}
	}
}

// SecretsProviderNamespace returns the namespace used for the secrets provider from the APP_NAMESPACE_NAME environment variable.
func SecretsProviderNamespace() string {
	if ns := os.Getenv("APP_NAMESPACE_NAME"); ns != "" {
		return ns
	}
	return "local-secrets-provider"
}

// ConjurNamespace returns the namespace used for Conjur from the CONJUR_NAMESPACE_NAME environment variable.
func ConjurNamespace() string {
	if ns := os.Getenv("CONJUR_NAMESPACE_NAME"); ns != "" {
		return ns
	}
	return "local-conjur"
}

// ClearBuffer resets the stdout and stderr buffers to empty.
func ClearBuffer(stdout *bytes.Buffer, stderr *bytes.Buffer) {
	stdout.Reset()
	stderr.Reset()
}

// WaitForSecretValue polls the pod environment to check if a secret value has been updated
// Returns nil when the expected value is found, or an error if timeout is reached
func WaitForSecretValue(client klient.Client, labelSelector string, container string, envVar string, expectedValue string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastStdout, lastStderr string
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s to contain %s. Last output: %q (stderr: %q)", envVar, expectedValue, lastStdout, lastStderr)
		case <-ticker.C:
			var stdout, stderr bytes.Buffer
			command := []string{"sh", "-c", fmt.Sprintf("printenv | grep %s || true", envVar)}
			err := RunCommandInSecretsProviderPod(client, labelSelector, container, command, &stdout, &stderr)
			lastStdout = stdout.String()
			lastStderr = stderr.String()

			// Check if command succeeded and output contains the expected value
			if err == nil && strings.Contains(lastStdout, expectedValue) {
				return nil
			}
		}
	}
}

// WaitForK8sSecretValue polls the K8s secret to check if it contains the expected key-value pair
// Returns nil when the expected value is found, or an error if timeout is reached
func WaitForK8sSecretValue(client klient.Client, namespace string, secretName string, key string, expectedValue string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastErr error
	var lastActualValue string
	var keyExists bool
	for {
		select {
		case <-ctx.Done():
			if !keyExists {
				return fmt.Errorf("timeout waiting for secret %s/%s key %s to exist. Last error: %v", namespace, secretName, key, lastErr)
			}
			return fmt.Errorf("timeout waiting for secret %s/%s key %s to contain %q. Actual value: %q. Last error: %v", namespace, secretName, key, expectedValue, lastActualValue, lastErr)
		case <-ticker.C:
			var secret corev1.Secret
			err := client.Resources(namespace).Get(context.TODO(), secretName, namespace, &secret)
			if err != nil {
				lastErr = err
				continue
			}

			// Check if the key exists and value matches
			if secretData, exists := secret.Data[key]; exists {
				keyExists = true
				actualValue := string(secretData)
				lastActualValue = actualValue
				if actualValue == expectedValue {
					return nil
				}
			} else {
				keyExists = false
			}
		}
	}
}

// WaitForK8sSecretValueToDelete polls the K8s secret to check if it does not contain the expected key
// Returns nil when the expected value is not found, or an error if timeout is reached
func WaitForK8sSecretValueToDelete(client klient.Client, namespace string, secretName string, key string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastErr error
	var lastActualValue string
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for secret %s/%s key %s to be deleted. Last value: %q. Last error: %v", namespace, secretName, key, lastActualValue, lastErr)
		case <-ticker.C:
			var secret corev1.Secret
			err := client.Resources(namespace).Get(context.TODO(), secretName, namespace, &secret)
			if err != nil {
				lastErr = err
				continue
			}

			// Check if the key exists
			if secretData, exists := secret.Data[key]; exists {
				lastActualValue = string(secretData)
				// If the key still exists, keep waiting for it to be deleted
				continue
			}
			return nil
		}
	}
}

// WaitForFileContent polls the pod to check if a file contains the expected content
// Returns nil when the expected content is found, or an error if timeout is reached
func WaitForFileContent(client klient.Client, labelSelector string, container string, filePath string, expectedContent string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for file %s to contain expected content", filePath)
		case <-ticker.C:
			var stdout, stderr bytes.Buffer
			command := []string{"cat", filePath}
			err := RunCommandInSecretsProviderPod(client, labelSelector, container, command, &stdout, &stderr)
			if err == nil && stdout.String() == expectedContent {
				return nil
			}
		}
	}
}

// WaitForFileDeleted polls the pod to check if a file has been deleted
// Returns nil when the file is deleted (empty output), or an error if timeout is reached
func WaitForFileDeleted(client klient.Client, labelSelector string, container string, filePath string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for file %s to be deleted", filePath)
		case <-ticker.C:
			var stdout, stderr bytes.Buffer
			command := []string{"cat", filePath}
			RunCommandInSecretsProviderPod(client, labelSelector, container, command, &stdout, &stderr)
			if stdout.String() == "" {
				return nil
			}
		}
	}
}

// DumpFailureDiagnosticsOnFailure writes focused namespace diagnostics for RCA when a test fails.
func DumpFailureDiagnosticsOnFailure(t *testing.T) {
	if !t.Failed() {
		return
	}

	namespace := SecretsProviderNamespace()
	t.Logf("Collecting failure diagnostics for namespace %q", namespace)

	dumpKubectlOutput(t, "pods-wide", "get", "pods", "-n", namespace, "-o", "wide")
	dumpKubectlOutput(t, "workloads", "get", "deploy,rs,job", "-n", namespace, "-o", "wide")
	dumpKubectlOutput(t, "events", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
	dumpKubectlOutput(t, "pods-describe", "describe", "pods", "-n", namespace)
	dumpKubectlOutput(t, "secrets-provider-logs", "logs", "-l", SPLabelSelector, "-c", LogsContainer, "--tail", "200", "-n", namespace)
	dumpKubectlOutput(t, "secrets-provider-previous-logs", "logs", "-l", SPLabelSelector, "-c", LogsContainer, "--previous", "--tail", "200", "-n", namespace)

	// Reloader pod failures are critical for reloader E2E and frequently require previous logs.
	dumpKubectlOutput(t, "reloader-deploy-describe", "describe", "deployment", "secrets-provider-reloader", "-n", namespace)
	dumpKubectlOutput(t, "reloader-pods", "get", "pods", "-n", namespace, "-l", "app.kubernetes.io/name=reloader", "-o", "wide")
	dumpKubectlOutput(t, "reloader-pods-describe", "describe", "pod", "-n", namespace, "-l", "app.kubernetes.io/name=reloader")
	dumpKubectlOutput(t, "reloader-logs", "logs", "-l", "app.kubernetes.io/name=reloader", "--all-containers", "--tail", "200", "-n", namespace)
	dumpKubectlOutput(t, "reloader-previous-logs", "logs", "-l", "app.kubernetes.io/name=reloader", "--all-containers", "--previous", "--tail", "200", "-n", namespace)
}

func dumpKubectlOutput(t *testing.T, section string, args ...string) {
	command := exec.Command("kubectl", args...)
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Logf("Diagnostics section %q failed: %v\nOutput:\n%s", section, err, string(output))
		return
	}
	t.Logf("Diagnostics section %q:\n%s", section, string(output))
}
