//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
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

func RunCommandInSecretsProviderPod(client klient.Client, command []string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	spPod, err := FetchPodWithLabelSelector(client, SecretsProviderNamespace(), SecretsProviderLabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets provider pod. %v", err)
	}

	if err = client.Resources(SecretsProviderNamespace()).ExecInPod(context.TODO(), SecretsProviderNamespace(), spPod.Name, TestAppContainer, command, stdout, stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	return nil
}

func FetchPodWithLabelSelector(client klient.Client, namespace string, labelSelector string) (v1.Pod, error) {
	var pods v1.PodList
	var pod v1.Pod

	err := client.Resources(namespace).List(context.TODO(), &pods, resources.WithLabelSelector(labelSelector))
	if err != nil {
		return pod, fmt.Errorf("failed to fetch pods. %v", err)
	}

	if len(pods.Items) == 1 {
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

func ConjurNamespace() string {
	if ns := os.Getenv("CONJUR_NAMESPACE_NAME"); ns != "" {
		return ns
	}
	return "local-conjur"
}
