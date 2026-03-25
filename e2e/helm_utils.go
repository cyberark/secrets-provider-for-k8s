//go:build e2e
// +build e2e

// Package e2e contains end-to-end tests for the Secrets Provider.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// AuthenticatorID returns the Conjur authenticator ID from the AUTHENTICATOR_ID environment variable, or a default value.
func AuthenticatorID() string {
	if id := os.Getenv("AUTHENTICATOR_ID"); id != "" {
		return id
	}
	return "authn-k8s/dev"
}

// ConjurAccount returns the Conjur account from the CONJUR_ACCOUNT environment variable, or a default value.
func ConjurAccount() string {
	if id := os.Getenv("CONJUR_ACCOUNT"); id != "" {
		return id
	}
	return "cucumber"
}

// ConjurApplianceURL returns the Conjur appliance URL based on the deployment configuration and namespace.
func ConjurApplianceURL() string {
	conjurNodeName := "conjur-follower"
	if deployment := os.Getenv("CONJUR_DEPLOYMENT"); deployment == "oss" {
		conjurNodeName = "conjur-oss"
	}
	conjurApplianceURL := fmt.Sprintf("https://%s.%s.svc.cluster.local", conjurNodeName, ConjurNamespace())
	if deployment := os.Getenv("CONJUR_DEPLOYMENT"); deployment == "dap" {
		conjurApplianceURL = conjurApplianceURL + "/api"
	}
	return conjurApplianceURL
}

// ConjurAuthnURL returns the full Conjur authentication URL for the configured authenticator.
func ConjurAuthnURL() string {
	return fmt.Sprintf("%s/authn-k8s/%s", ConjurApplianceURL(), AuthenticatorID())
}

// GetImagePath returns the Docker image path for the secrets provider based on the deployment platform.
func GetImagePath() string {
	imagePath := SecretsProviderNamespace()

	// KinD runs load images directly into the cluster (no registry push/pull).
	// Avoid relying on DOCKER_REGISTRY_* env vars in that mode.
	if strings.ToLower(os.Getenv("TEST_PLATFORM")) == "kind" {
		return imagePath
	}

	if os.Getenv("PLATFORM") == "openshift" && os.Getenv("DEV") == "false" {
		imagePath = fmt.Sprintf("%s/%s", os.Getenv("PULL_DOCKER_REGISTRY_PATH"), os.Getenv("APP_NAMESPACE_NAME"))
	} else if os.Getenv("PLATFORM") == "kubernetes" && os.Getenv("DEV") == "false" {
		imagePath = fmt.Sprintf("%s/%s", os.Getenv("DOCKER_REGISTRY_PATH"), os.Getenv("APP_NAMESPACE_NAME"))
	}
	return imagePath
}

// FillHelmChart generates a Helm values file by populating a template with environment-specific replacements.
func FillHelmChart(client klient.Client, id string, optReplacements map[string]string) (string, error) {
	uniqueTestID := os.Getenv("UNIQUE_TEST_ID")
	chartName := fmt.Sprintf("%stest-values-%s.yaml", id, uniqueTestID)
	chartPath := filepath.Join("..", "helm", "secrets-provider", "ci", chartName)

	templateName := fmt.Sprintf("test-values-template.yaml")
	templatePath := filepath.Join("..", "helm", "secrets-provider", "ci", templateName)

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("error reading template file: %v", err)
	}

	conjurCert, err := FetchConjurServerCert(client)
	if err != nil {
		return "", fmt.Errorf("failed to fetch conjur server cert. %v", err)
	}

	// Perform replacements using environment variables or defaults - default replacements
	replacements := map[string]string{
		"SECRETS_PROVIDER_ROLE":           getEnvOrDefault("SECRETS_PROVIDER_ROLE", "secrets-provider-role"),
		"SECRETS_PROVIDER_ROLE_BINDING":   getEnvOrDefault("SECRETS_PROVIDER_ROLE_BINDING", "secrets-provider-role-binding"),
		"CREATE_SERVICE_ACCOUNT":          getEnvOrDefault("CREATE_SERVICE_ACCOUNT", "true"),
		"SERVICE_ACCOUNT":                 getEnvOrDefault("SERVICE_ACCOUNT", "secrets-provider-service-account"),
		"K8S_SECRETS":                     getEnvOrDefault("K8S_SECRETS", "test-k8s-secret"),
		"CONJUR_ACCOUNT":                  getEnvOrDefault("CONJUR_ACCOUNT", "cucumber"),
		"CONJUR_APPLIANCE_URL":            getEnvOrDefault("CONJUR_APPLIANCE_URL", "https://conjur-follower."+ConjurNamespace()+".svc.cluster.local/api"),
		"CONJUR_AUTHN_URL":                getEnvOrDefault("CONJUR_AUTHN_URL", "https://conjur-follower."+ConjurNamespace()+".svc.cluster.local/api/authn-k8s/"+AuthenticatorID()),
		"CONJUR_AUTHN_LOGIN":              getEnvOrDefault("CONJUR_AUTHN_LOGIN", "host/conjur/authn-k8s/"+AuthenticatorID()+"/apps/"+SecretsProviderNamespace()+"/*/*"),
		"SECRETS_PROVIDER_SSL_CONFIG_MAP": getEnvOrDefault("SECRETS_PROVIDER_SSL_CONFIG_MAP", "secrets-provider-ssl-config-map"),
		"IMAGE_PULL_POLICY":               getEnvOrDefault("IMAGE_PULL_POLICY", "IfNotPresent"),
		"IMAGE":                           getEnvOrDefault("IMAGE", GetImagePath()+"/secrets-provider"),
		"TAG":                             getEnvOrDefault("TAG", "latest"),
		"LABELS":                          getEnvOrDefault("LABELS", "app: test-helm"),
		"DEBUG":                           getEnvOrDefault("DEBUG", "false"),
		"LOG_LEVEL":                       getEnvOrDefault("LOG_LEVEL", "info"),
		"RETRY_COUNT_LIMIT":               getEnvOrDefault("RETRY_COUNT_LIMIT", "5"),
		"RETRY_INTERVAL_SEC":              getEnvOrDefault("RETRY_INTERVAL_SEC", "5"),
		"CONJUR_SSL_CERTIFICATE":          getEnvOrDefault("CONJUR_SSL_CERTIFICATE", conjurCert),
		"IMAGE_PULL_SECRET":               getEnvOrDefault("IMAGE_PULL_SECRET", ""),
		"RELOADER_ENABLED":                getEnvOrDefault("RELOADER_ENABLED", "false"),
		"RELOADER_DEPLOYMENT_REPLICAS":    getEnvOrDefault("RELOADER_DEPLOYMENT_REPLICAS", "1"),
		"RELOADER_ENABLE_HA":              getEnvOrDefault("RELOADER_ENABLE_HA", "false"),
	}

	// OPTIONAL - update default replacements with desired values
	for key, value := range optReplacements {
		_, exists := replacements[key]
		if exists {
			replacements[key] = value
		}
	}

	for key, value := range replacements {
		templateContent = bytesReplace(templateContent, "{{ "+key+" }}", value)
	}

	// Write the modified content to the target file
	err = os.WriteFile(chartPath, templateContent, 0644)
	if err != nil {
		return "", fmt.Errorf("error writing to target file: %v", err)
	}

	return chartPath, nil
}

// FillHelmChartNoOverrideDefaults generates a Helm values file using only default values without overriding template defaults.
func FillHelmChartNoOverrideDefaults(client klient.Client, id string, optReplacements map[string]string) (string, error) {
	uniqueTestID := os.Getenv("UNIQUE_TEST_ID")
	chartName := fmt.Sprintf("%stake-default-test-values-%s.yaml", id, uniqueTestID)
	chartPath := filepath.Join("..", "helm", "secrets-provider", "ci", chartName)

	templateName := fmt.Sprintf("take-default-test-values-template.yaml")
	templatePath := filepath.Join("..", "helm", "secrets-provider", "ci", templateName)

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("error reading template file: %v", err)
	}

	// Perform replacements using environment variables or defaults - default replacements
	replacements := map[string]string{
		"K8S_SECRETS":          getEnvOrDefault("K8S_SECRETS", "test-k8s-secret"),
		"CONJUR_ACCOUNT":       getEnvOrDefault("CONJUR_ACCOUNT", "cucumber"),
		"LABELS":               getEnvOrDefault("LABELS", "app: test-helm"),
		"CONJUR_APPLIANCE_URL": getEnvOrDefault("CONJUR_APPLIANCE_URL", "https://conjur-follower."+ConjurNamespace()+".svc.cluster.local/api"),
		"CONJUR_AUTHN_URL":     getEnvOrDefault("CONJUR_AUTHN_URL", "https://conjur-follower."+ConjurNamespace()+".svc.cluster.local/api/authn-k8s/"+AuthenticatorID()),
		"CONJUR_AUTHN_LOGIN":   getEnvOrDefault("CONJUR_AUTHN_LOGIN", "host/conjur/authn-k8s/"+AuthenticatorID()+"/apps/"+SecretsProviderNamespace()+"/*/*"),
	}

	// OPTIONAL - update default replacements with desired values
	for key, value := range optReplacements {
		_, exists := replacements[key]
		if exists {
			replacements[key] = value
		}
	}

	for key, value := range replacements {
		templateContent = bytesReplace(templateContent, "{{ "+key+" }}", value)
	}

	// Write the modified content to the target file
	err = os.WriteFile(chartPath, templateContent, 0644)
	if err != nil {
		return "", fmt.Errorf("error writing to target file: %v", err)
	}

	return chartPath, nil
}

// FillHelmChartTestImage generates a Helm values file focused on image configuration for test deployments.
func FillHelmChartTestImage(client klient.Client, id string, optReplacements map[string]string) (string, error) {
	uniqueTestID := os.Getenv("UNIQUE_TEST_ID")
	chartName := fmt.Sprintf("%stake-image-values-%s.yaml", id, uniqueTestID)
	chartPath := filepath.Join("..", "helm", "secrets-provider", "ci", chartName)

	templateName := fmt.Sprintf("take-image-values-template.yaml")
	templatePath := filepath.Join("..", "helm", "secrets-provider", "ci", templateName)

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("error reading template file: %v", err)
	}

	// Perform replacements using environment variables or defaults - default replacements
	replacements := map[string]string{
		"IMAGE":             getEnvOrDefault("IMAGE", GetImagePath()+"/secrets-provider"),
		"TAG":               getEnvOrDefault("TAG", "latest"),
		"IMAGE_PULL_POLICY": getEnvOrDefault("IMAGE_PULL_POLICY", "IfNotPresent"),
	}

	// OPTIONAL - update default replacements with desired values
	for key, value := range optReplacements {
		_, exists := replacements[key]
		if exists {
			replacements[key] = value
		}
	}

	for key, value := range replacements {
		templateContent = bytesReplace(templateContent, "{{ "+key+" }}", value)
	}

	// Write the modified content to the target file
	err = os.WriteFile(chartPath, templateContent, 0644)
	if err != nil {
		return "", fmt.Errorf("error writing to target file: %v", err)
	}

	return chartPath, nil
}

// DeployTestAppWithHelm deploys a test application that reads secrets from a Kubernetes Secret.
func DeployTestAppWithHelm(client klient.Client, id string) error {
	// create Deployment
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
	d, err := GetDeployment(client, id+"test-env")
	if err != nil {
		return err
	}
	err = WaitResourceScaled(client, d, 1)
	if err != nil {
		return err
	}
	return nil
}

// CreateK8sRole creates the ServiceAccount, Role, and RoleBinding required for the secrets provider deployment.
func CreateK8sRole(client klient.Client, id string) error {
	// create ServiceAccount
	serviceAccount := corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      id + "secrets-provider-service-account",
			Namespace: SecretsProviderNamespace(),
		},
	}
	err := client.Resources().Create(context.TODO(), &serviceAccount)
	if err != nil {
		return err
	}

	// create Role
	role := rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      id + "secrets-provider-role",
			Namespace: SecretsProviderNamespace(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "update"},
			},
		},
	}
	err = client.Resources().Create(context.TODO(), &role)
	if err != nil {
		return err
	}

	// create RoleBinding
	roleBinding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      id + "secrets-provider-role-binding",
			Namespace: SecretsProviderNamespace(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: id + "secrets-provider-service-account",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     id + "secrets-provider-role",
		},
	}
	err = client.Resources().Create(context.TODO(), &roleBinding)
	if err != nil {
		return err
	}

	return nil
}

// CreateK8sSecretForHelmDeployment creates a Kubernetes Secret used for Helm-based test deployments.
func CreateK8sSecretForHelmDeployment(client klient.Client) error {
	// create Secret
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-test-k8s-secret",
			Namespace: SecretsProviderNamespace(),
		},
		StringData: map[string]string{
			"conjur-map": "secret: data/secrets/another_test_secret",
		},
		Type: "Opaque",
	}
	err := client.Resources().Create(context.TODO(), &secret)
	if err != nil {
		return err
	}
	return nil
}

// FetchConjurServerCert retrieves the Conjur server TLS certificate from the running Conjur pod.
func FetchConjurServerCert(client klient.Client) (string, error) {
	label := ConjurFollowerLabelSelector
	certLocation := "/opt/conjur/etc/ssl/conjur.pem"
	container := ConjurClusterContainer
	if os.Getenv("CONJUR_DEPLOYMENT") == "oss" {
		label = ConjurCLILabelSelector
		certLocation = "/home/cli/conjur-server.pem"
		container = ConjurCLIContainer
	}

	pod, err := FetchPodWithLabelSelector(client, ConjurNamespace(), label)
	if err != nil {
		return "", fmt.Errorf("failed to fetch conjur pod. %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := []string{"cat", certLocation}
	if err := client.Resources(ConjurNamespace()).ExecInPod(context.TODO(), ConjurNamespace(), pod.Name, container, command, &stdout, &stderr); err != nil {
		return "", fmt.Errorf("failed to execute command. %v, %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// getEnvOrDefault returns the value of the environment variable if it exists, or a default value otherwise.
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// bytesReplace replaces all occurrences of old with new in the input byte slice.
func bytesReplace(input []byte, old, new string) []byte {
	return []byte(strings.ReplaceAll(string(input), old, new))
}

// CreateConjurCertFile writes the Conjur server certificate to a local PEM file for Helm chart use.
func CreateConjurCertFile(client klient.Client) error {
	conjurCert, err := FetchConjurServerCert(client)
	if err != nil {
		return err
	}
	file, err := os.Create("conjur-server.pem")
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(conjurCert)
	if err != nil {
		return err
	}
	return nil
}

// DeploySecretsProviderJobWithHelm installs the secrets provider via Helm and waits for the job to complete.
func DeploySecretsProviderJobWithHelm(cfg *envconf.Config, id string, chartPaths ...string) error {
	// set conjur cert to file
	err := CreateConjurCertFile(cfg.Client())
	if err != nil {
		return err
	}

	// Ensure each test run starts from a fresh Helm release state.
	manager := helm.New(cfg.KubeconfigFile())
	spName := id + "secrets-provider"
	uninstallErr := manager.RunUninstall(
		helm.WithReleaseName(spName),
	)
	if uninstallErr != nil {
		uninstallErrMessage := strings.ToLower(uninstallErr.Error())
		if !strings.Contains(uninstallErrMessage, "not found") && !strings.Contains(uninstallErrMessage, "release: not found") {
			return uninstallErr
		}
	}

	// helm upgrade --install
	if len(chartPaths) == 2 {
		err = manager.RunUpgrade(
			helm.WithName(spName),
			helm.WithChart("../helm/secrets-provider"),
			helm.WithArgs("-f", chartPaths[0]),
			helm.WithArgs("-f", chartPaths[1]),
			helm.WithArgs("--install"),
			helm.WithArgs("--set-file", fmt.Sprintf("environment.conjur.sslCertificate.value=%s", "./conjur-server.pem")),
		)
		if err != nil {
			return err
		}
	} else {
		err = manager.RunUpgrade(
			helm.WithName(spName),
			helm.WithChart("../helm/secrets-provider"),
			helm.WithArgs("-f", chartPaths[0]),
			helm.WithArgs("--install"),
			helm.WithArgs("--set-file", fmt.Sprintf("environment.conjur.sslCertificate.value=%s", "./conjur-server.pem")),
		)
		if err != nil {
			return err
		}
	}

	// wait for job completion
	job, err := GetJob(cfg.Client(), spName)
	if err != nil {
		return err
	}

	err = WaitJobCompleted(cfg.Client(), job)
	if err != nil {
		_ = CleanChartPathFiles(chartPaths)
		return err
	}

	err = CleanChartPathFiles(chartPaths)
	if err != nil {
		return err
	}

	return nil
}

// RemoveJobWithHelm uninstalls a Helm release by name.
func RemoveJobWithHelm(cfg *envconf.Config, name string) error {
	manager := helm.New(cfg.KubeconfigFile())
	err := manager.RunUninstall(
		helm.WithReleaseName(name),
	)
	if err != nil {
		return err
	}

	return nil
}

// GetPodLogs retrieves the logs from a specific container in a pod.
func GetPodLogs(client klient.Client, podName string, namespace string, container string) (*bytes.Buffer, error) {
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return nil, fmt.Errorf("unable to initialize K8s client: %v", err)
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
	})
	logs, err := req.Stream(context.TODO())
	if err != nil {
		return nil, fmt.Errorf(
			"failed to stream logs from container %s in pod %s in namespace %s: %v",
			container, podName, namespace, err,
		)
	}
	defer logs.Close()

	content := new(bytes.Buffer)
	_, err = io.Copy(content, logs)
	if err != nil {
		return nil, fmt.Errorf("unable to copy log stream to buffer: %v", err)
	}

	return content, nil
}

// GetTimestamp parses a timestamp from a log string by searching for a line containing the given token.
func GetTimestamp(logs string, token string) (time.Time, error) {
	logsLines := strings.Split(logs, "\n")

	for _, line := range logsLines {
		if strings.Contains(line, token) {
			parse := strings.Fields(line)
			if len(parse) < 3 {
				continue
			}

			stamp := parse[1] + " " + parse[2]
			currentTime, err := time.Parse("2006/01/02 15:04:05.000000", stamp)
			if err != nil {
				continue
			}

			return currentTime, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not find token %s", token)
}

// ConfigDir returns the platform-specific configuration directory path.
func ConfigDir() string {
	if os.Getenv("PLATFORM") == "openshift" {
		return "config/openshift"
	}
	return "config/k8s"
}

// CleanChartPathFiles removes the generated Helm chart files and the Conjur certificate file.
func CleanChartPathFiles(paths []string) error {
	for _, path := range paths {
		err := os.Remove(path)
		if err != nil {
			return err
		}
	}
	err := os.Remove("./conjur-server.pem")
	if err != nil {
		return err
	}
	return nil
}
