//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

func ReloadWithTemplate(client klient.Client, template string) error {
	fmt.Println("Reloading test environment with template " + template)
	os.Setenv("TEMPLATE", template)
	cmd := exec.Command("../deploy/redeploy.sh")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error running redeploy.sh. Stdout:\n" + string(out))
		return err
	}

	fmt.Println("Waiting for secrets provider pod to be ready...")
	pod, err := FetchPodWithLabelSelector(client, SecretsProviderNamespace(), SecretsProviderLabelSelector)
	if err != nil {
		fmt.Println("Error locating Secrets Provider pod after redeploy. Stdout:\n" + string(out))
		return err
	}

	err = wait.For(conditions.New(client.Resources()).PodReady(k8s.Object(&pod)), wait.WithTimeout(time.Minute*1))
	if err != nil {
		fmt.Println("Error waiting for Secrets Provider pod to be 'Ready' after redeploy. Stdout:\n" + string(out))
		return err
	}

	return nil
}

func ScaleDeployment(client klient.Client, deploymentName string, namespace string, replicas int32) error {
	mergePatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to json marshal: %v", err)
	}

	deployment := GetDeployment(deploymentName)
	err = client.Resources(namespace).Patch(context.TODO(), deployment, k8s.Patch{PatchType: types.StrategicMergePatchType, Data: mergePatch})
	if err != nil {
		return fmt.Errorf("failed to patch deployment: %v", err)
	}

	fmt.Println("waiting for deployment to be scaled")
	if replicas > 0 {
		err = wait.For(conditions.New(client.Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
			return object.(*appsv1.Deployment).Status.ReadyReplicas
		}, replicas), wait.WithTimeout(time.Minute*1))
		if err != nil {
			return fmt.Errorf("failed to wait for scaling: %v", err)
		}
	} else {
		pods, err := GetPods(client, namespace, SecretsProviderLabelSelector)
		if err != nil {
			return err
		}
		for _, pod := range pods.Items {
			err = wait.For(conditions.New(client.Resources()).ResourceDeleted(&pod), wait.WithTimeout(time.Minute*1))
			if err != nil {
				return fmt.Errorf("failed to wait for deletion: %v", err)
			}
		}
	}
	fmt.Println("deployment successfully scaled")

	return nil
}

func GetDeployment(deploymentName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: SecretsProviderNamespace(),
			Labels: map[string]string{
				"app": "test-env",
			},
		},
	}
}

func GetPods(client klient.Client, namespace string, labelSelector string) (corev1.PodList, error) {
	var pods corev1.PodList
	err := client.Resources(namespace).List(context.TODO(), &pods, resources.WithLabelSelector(labelSelector))
	if err != nil {
		return corev1.PodList{}, fmt.Errorf("failed to fetch pods: %v", err)
	}

	return pods, nil
}

func SetConjurSecret(client klient.Client, varId string, value string) error {
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), CLILabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "set", "-i", varId, "-v", value}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, CLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Value added") {
		return fmt.Errorf("failed to set secret")
	}
	return nil
}

func GetConjurSecret(client klient.Client, varId string) (string, error) {
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), CLILabelSelector)
	if err != nil {
		return "", fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "get", "-i", varId}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, CLIContainer, command, &stdout, &stderr); err != nil {
		return "", fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func RunCommandInSecretsProviderPod(client klient.Client, command []string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	spPod, err := FetchPodWithLabelSelector(client, SecretsProviderNamespace(), SecretsProviderLabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets provider pod. %v", err)
	}

	err = RunCommandInPod(client, command, spPod.Name, stdout, stderr)
	if err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	return nil
}

func RunCommandInPod(client klient.Client, command []string, podName string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	if err := client.Resources(SecretsProviderNamespace()).ExecInPod(context.TODO(), SecretsProviderNamespace(), podName, TestAppContainer, command, stdout, stderr); err != nil {
		return err
	}

	return nil
}

func FetchPodWithLabelSelector(client klient.Client, namespace string, labelSelector string) (corev1.Pod, error) {
	var pods corev1.PodList
	var pod corev1.Pod

	err := client.Resources(namespace).List(context.TODO(), &pods, resources.WithLabelSelector(labelSelector))
	if err != nil {
		return pod, fmt.Errorf("failed to fetch pods. %v", err)
	}

	if len(pods.Items) >= 1 {
		return pods.Items[0], nil
	}

	return pod, fmt.Errorf("Expected exactly 1 pod to match label selector %s in namespace %s. Matching pod list: %v", labelSelector, namespace, pods.Items)
}

func SecretsProviderNamespace() string {
	if ns := os.Getenv("APP_NAMESPACE_NAME"); ns != "" {
		return ns
	}
	return "local-secrets-provider"
}

func SecretsProviderPodName(client klient.Client) string {
	spPod, err := FetchPodWithLabelSelector(client, SecretsProviderNamespace(), SecretsProviderLabelSelector)
	if err != nil {
		return ""
	}

	return spPod.Name
}

func ConjurNamespace() string {
	if ns := os.Getenv("CONJUR_NAMESPACE_NAME"); ns != "" {
		return ns
	}
	return "local-conjur"
}
