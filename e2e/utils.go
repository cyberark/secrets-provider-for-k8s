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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

func ReloadWithTemplate(client klient.Client, template string) (v1.Pod, error) {
	fmt.Println("Reloading test environment with template " + template)
	os.Setenv("TEMPLATE", template)
	cmd := exec.Command("../deploy/redeploy.sh")

	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()

	fmt.Println(string(out))
	if err != nil {
		fmt.Println("Error: " + string(out))
		return v1.Pod{}, fmt.Errorf("failed to execute command. %v, %s", err, out)
	}

	fmt.Println("Waiting for secrets provider pod to be ready...")

	// Get the Secrets Provider Pod
	var pods v1.PodList
	FetchPodsFromNamespace(client, SecretsProviderNamespace(), &pods)

	// Wait for the Secrets Provider Pod to be ready
	pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pods.Items[0].Name, Namespace: SecretsProviderNamespace()}}
	err = wait.For(conditions.New(client.Resources()).PodReady(k8s.Object(&pod)), wait.WithTimeout(time.Minute*1))

	if err != nil {
		fmt.Println("Stdout:\n" + string(out))
		return v1.Pod{}, fmt.Errorf("error waiting for PodReady: %v", err)
	}

	return FetchSecretsProviderPod(client)
}

func SetConjurSecret(client klient.Client, varId string, value string) error {
	// Get the CLI Pod
	var pods v1.PodList
	FetchPodsFromNamespace(client, ConjurNamespace(), &pods)

	podName := pods.Items[0].Name

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "set", "-i", varId, "-v", value}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), podName, CLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Value added") {
		return fmt.Errorf("failed to set secret")
	}
	return nil
}

func RunCommandInSecretsProviderPod(client klient.Client, spPodName string, command []string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	// Get the Secrets Provider Pod
	var pods v1.PodList
	FetchPodsFromNamespace(client, SecretsProviderNamespace(), &pods)

	if err := client.Resources(SecretsProviderNamespace()).ExecInPod(context.TODO(), SecretsProviderNamespace(), spPodName, TestAppContainer, command, stdout, stderr); err != nil {
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

func FetchSecretsProviderPod(client klient.Client) (v1.Pod, error) {
	var pods v1.PodList
	var spPod v1.Pod
	for {
		FetchPodsFromNamespace(client, SecretsProviderNamespace(), &pods)
		// In openshift, there is a deployment pod in the namespace we need to ignore
		if len(pods.Items) == 2 {
			for _, pod := range pods.Items {
				if pod.Name != "test-env-1-deploy" {
					return pod, nil
				}
			}
			return spPod, fmt.Errorf("Unable to locate secrets provider pod in the podlist: %v", pods.Items)
		}

		if len(pods.Items) == 1 {
			return pods.Items[0], nil
		}

		fmt.Println("No pods found, sleeping for 5 seconds...")
		time.Sleep(5 * time.Second)
	}
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
