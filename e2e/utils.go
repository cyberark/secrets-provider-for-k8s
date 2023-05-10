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
	cmd1 := exec.Command("../bin/start", "--dev", "--reload", "--template="+template)
	out, err := cmd1.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, out)
	}

	// Get the Secrets Provider Pod
	var pods v1.PodList
	err = client.Resources(SecretsProviderNamespace).List(context.TODO(), &pods)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Printf("found %d pods in namespace %s", len(pods.Items), SecretsProviderNamespace)
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods in namespace %s", SecretsProviderNamespace)
	}

	// Wait for the Secrets Provider Pod to be ready
	pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pods.Items[0].Name}}
	err = wait.For(conditions.New(client.Resources()).PodReady(k8s.Object(&pod)), wait.WithTimeout(time.Minute*1))
	if err != nil {
		return fmt.Errorf("error waiting for PodReady: %v", err)
	}

	// Reload complete
	fmt.Println("Reload complete")

	return nil
}

func SetConjurSecret(client klient.Client, varId string, value string) error {
	// Get the CLI Pod
	var pods v1.PodList
	err := client.Resources(ConjurNamespace).List(context.TODO(), &pods)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Printf("found %d pods in namespace %s", len(pods.Items), SecretsProviderNamespace)
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods in namespace %s", SecretsProviderNamespace)
	}

	podName := pods.Items[0].Name

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "set", "-i", varId, "-v", value}
	if err := client.Resources().ExecInPod(context.TODO(), ConjurNamespace, podName, CLIContainer, command, &stdout, &stderr); err != nil {
		fmt.Print(stderr.String())
		fmt.Print(err)
	}

	if !strings.Contains(stdout.String(), "Value added") {
		return fmt.Errorf("failed to set secret")
	}

	fmt.Println("Value added")

	return nil
}

func RunCommandInSecretsProviderPod(client klient.Client, command []string, stdout *bytes.Buffer, stderr *bytes.Buffer) {
	var pods v1.PodList
	err := client.Resources(SecretsProviderNamespace).List(context.TODO(), &pods)
	if err != nil {
		fmt.Print(err)
	}
	if len(pods.Items) == 0 {
		fmt.Printf("Error: no pods in namespace %s", SecretsProviderNamespace)
	}
	podName := pods.Items[0].Name

	if err := client.Resources(SecretsProviderNamespace).ExecInPod(context.TODO(), SecretsProviderNamespace, podName, TestContainer, command, stdout, stderr); err != nil {
		fmt.Print(stderr.String())
		fmt.Print(err)
	}
}
