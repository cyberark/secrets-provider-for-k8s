//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

func ReloadWithTemplate(client klient.Client, template string) error {
	fmt.Println("Reloading test environment with template " + template)
	cmd1 := exec.Command("../bin/start", "--dev", "--reload", "--template="+template)
	out, err := cmd1.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, out)
	}

	// Get the Secrets Provider Pod
	var pods v1.PodList
	FetchPodsFromNamespace(client, SecretsProviderNamespace, &pods)

	// Wait for the Secrets Provider Pod to be ready
	pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pods.Items[0].Name}}
	err = wait.For(conditions.New(client.Resources(SecretsProviderNamespace)).PodReady(k8s.Object(&pod)), wait.WithTimeout(time.Minute*1))
	if err != nil {
		return fmt.Errorf("error waiting for PodReady: %v", err)
	}

	fmt.Println("Reload complete")

	return nil
}

func SetConjurSecret(client klient.Client, varId string, value string) error {
	// Get the CLI Pod
	var pods v1.PodList
	FetchPodsFromNamespace(client, ConjurNamespace, &pods)

	podName := pods.Items[0].Name

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "set", "-i", varId, "-v", value}
	if err := client.Resources(ConjurNamespace).ExecInPod(context.TODO(), ConjurNamespace, podName, CLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Value added") {
		return fmt.Errorf("failed to set secret")
	}

	fmt.Println("Value added")

	return nil
}

func RunCommandInSecretsProviderPod(client klient.Client, command []string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	// Get the Secrets Provider Pod
	var pods v1.PodList
	FetchPodsFromNamespace(client, SecretsProviderNamespace, &pods)
	podName := pods.Items[0].Name

	if err := client.Resources(SecretsProviderNamespace).ExecInPod(context.TODO(), SecretsProviderNamespace, podName, TestAppContainer, command, stdout, stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	return nil
}

func FetchPodsFromNamespace(client klient.Client, namespace string, pods *v1.PodList) error {
	err := client.Resources(namespace).List(context.TODO(), pods)
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found in namespace %s", namespace)
	}
	fmt.Printf("Found %d pod(s) in namespace %s\n", len(pods.Items), namespace)
	return nil
}
