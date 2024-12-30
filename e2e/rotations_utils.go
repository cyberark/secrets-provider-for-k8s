//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/e2e-framework/klient"
)

func DeleteTestSecret(client klient.Client) error {
	err := LoadPolicy(client, "conjur-delete-secret")
	if err != nil {
		return err
	}
	return nil
}

func RestoreTestSecret(client klient.Client) error {
	// Restore the actual secret object by reloading the policy
	err := LoadPolicy(client, "conjur-secrets")
	if err != nil {
		return err
	}
	// Restore the default value
	err = SetConjurSecret(client, "secrets/test_secret", "supersecret")
	if err != nil {
		return err
	}
	return nil
}

func CreateTestingDirectories(client klient.Client) error {
	// create 'generated' directory for generated policies (current /generated is .gitignored)
	cmd := exec.Command("mkdir", "../deploy/policy/generated")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create 'generated' directory: %v", err)
	}

	// create 'policy' directory in conjur
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), ConjurCLILabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"mkdir", "tmp/policy"}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, ConjurCLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to create 'policy' directory in CLI container. %v, %s", err, stderr.String())
	}

	return nil
}

func DeleteTestingDirectories(client klient.Client) error {
	// delete 'generated' directory for generated policies (current /generated is .gitignored)
	cmd := exec.Command("rm", "-rf", "../deploy/policy/generated")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to remove 'generated' directory: %v", err)
	}

	// delete 'policy' directory in conjur
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), ConjurCLILabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"rm", "-rf", "tmp/policy"}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, ConjurCLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to remove 'tmp/policy' directory in CLI container. %v, %s", err, stderr.String())
	}

	return nil
}

func GeneratePolicyFromTemplate(client klient.Client, filename string) (string, string, error) {
	// retrieve and execute template
	templateName := fmt.Sprintf("%s.template.sh.yml", filename)
	templatePath := filepath.Join("..", "deploy", "policy", "templates", templateName)
	cmd := exec.Command(templatePath)

	// set output file
	fileName := fmt.Sprintf("%s.%s.yml", SecretsProviderNamespace(), filename)
	filePath := filepath.Join("..", "deploy", "policy", "generated", fileName)

	// generate policy
	file, err := os.Create(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create file: %v", err)
	}

	// redirect template contents to policy
	cmd.Stdout = file
	err = cmd.Start()
	if err != nil {
		return "", "", fmt.Errorf("failed to redirect output: %v", err)
	}

	// cmd.Wait() waits for any copying to stdin or copying from stdout or stderr to complete
	err = cmd.Wait()
	if err != nil {
		return "", "", fmt.Errorf("failed to wait for output redirection: %v", err)
	}

	return fileName, filePath, nil
}

func CopyFileIntoPod(client klient.Client, podName string, namespace string, containerName string, src string, dst string) error {
	// create client-go clientset
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return fmt.Errorf("unable to initialize K8s client: %v", err)
	}

	// open the file to copy
	localFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer localFile.Close()

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve pod: %v", err)
	}

	// validate container existsin pod
	var container *corev1.Container
	for _, c := range pod.Spec.Containers {
		if c.Name == containerName {
			container = &c
			break
		}
	}

	if container == nil {
		return fmt.Errorf("failed to find container in pod: %v", err)
	}

	// create a stream to the container
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName)

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   []string{"bash", "-c", "cat > " + dst},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	execute, err := remotecommand.NewSPDYExecutor(client.RESTConfig(), "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %v", err)
	}

	// Create a stream to the container
	err = execute.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  localFile,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return fmt.Errorf("failed to stream: %v", err)
	}

	return nil
}

func LoadPolicy(client klient.Client, filename string) error {
	// create generated file from template
	fileName, src, err := GeneratePolicyFromTemplate(client, filename)
	if err != nil {
		return err
	}

	// copy policy into conjur pod
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), ConjurCLILabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var dst string = "tmp/policy/" + fileName
	err = CopyFileIntoPod(client, pod.Name, ConjurNamespace(), ConjurCLIContainer, src, dst)
	if err != nil {
		return err
	}

	// update root with policy
	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "policy", "update", "-b", "root", "-f", dst}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, ConjurCLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to update policy root in conjur: %v, %s", err, stderr.String())
	}

	return nil
}
