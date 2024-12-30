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
	pod, err := FetchPodWithLabelSelector(client, SecretsProviderNamespace(), SPLabelSelector)
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

func WaitResourceScaled(client klient.Client, deployment *appsv1.Deployment, replicas int32) error {
	err := wait.For(conditions.New(client.Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, replicas), wait.WithTimeout(time.Minute*3))
	if err != nil {
		return fmt.Errorf("failed to wait for scaling: %v", err)
	}
	return nil
}

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

func WaitJobCompleted(client klient.Client, job *batchv1.Job) error {
	err := wait.For(conditions.New(client.Resources()).JobCompleted(job), wait.WithTimeout(time.Minute*1))
	if err != nil {
		return fmt.Errorf("failed to wait for job completion: %v", err)
	}
	return nil
}

func GetDeployment(client klient.Client, deploymentName string) (*appsv1.Deployment, error) {
	var deployment appsv1.Deployment
	err := client.Resources().Get(context.TODO(), deploymentName, SecretsProviderNamespace(), &deployment)
	if err != nil {
		return &deployment, err
	}
	return &deployment, nil
}

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

func GetJob(client klient.Client, jobName string) (*batchv1.Job, error) {
	var job batchv1.Job
	err := client.Resources().Get(context.TODO(), jobName, SecretsProviderNamespace(), &job)
	if err != nil {
		return &job, err
	}
	return &job, nil
}

func GetSecret(client klient.Client, secretName string) (*corev1.Secret, error) {
	var secret corev1.Secret
	err := client.Resources().Get(context.TODO(), secretName, SecretsProviderNamespace(), &secret)
	if err != nil {
		return &secret, err
	}
	return &secret, nil
}

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

func GetServiceAccount(client klient.Client, saName string) (*corev1.ServiceAccount, error) {
	var sa corev1.ServiceAccount
	err := client.Resources().Get(context.TODO(), saName, SecretsProviderNamespace(), &sa)
	if err != nil {
		return &sa, err
	}
	return &sa, nil
}

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

func GetConfigMap(client klient.Client, configMapName string) (*corev1.ConfigMap, error) {
	var configMap corev1.ConfigMap
	err := client.Resources().Get(context.TODO(), configMapName, SecretsProviderNamespace(), &configMap)
	if err != nil {
		return &configMap, err
	}
	return &configMap, nil
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
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), ConjurCLILabelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "set", "-i", varId, "-v", value}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, ConjurCLIContainer, command, &stdout, &stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Value added") {
		return fmt.Errorf("failed to set secret")
	}
	return nil
}

func GetConjurSecret(client klient.Client, varId string) (string, error) {
	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), ConjurCLILabelSelector)
	if err != nil {
		return "", fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"conjur", "variable", "get", "-i", varId}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, ConjurCLIContainer, command, &stdout, &stderr); err != nil {
		return "", fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func RunCommandInSecretsProviderPod(client klient.Client, labelSelector string, container string, command []string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	pod, err := FetchPodWithLabelSelector(client, SecretsProviderNamespace(), labelSelector)
	if err != nil {
		return fmt.Errorf("failed to fetch cli pod. %v", err)
	}

	if err := client.Resources(SecretsProviderNamespace()).ExecInPod(context.TODO(), SecretsProviderNamespace(), pod.Name, container, command, stdout, stderr); err != nil {
		return fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
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

func ConjurNamespace() string {
	if ns := os.Getenv("CONJUR_NAMESPACE_NAME"); ns != "" {
		return ns
	}
	return "local-conjur"
}

func ClearBuffer(stdout *bytes.Buffer, stderr *bytes.Buffer) {
	stdout.Reset()
	stderr.Reset()
}
