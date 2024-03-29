//go:build e2e
// +build e2e

package e2e

const (
	// Container names
	TestAppContainer             = "test-app"
	CLIContainer                 = "conjur-cli"
	SecretsProviderLabelSelector = "app=test-env"
	CLILabelSelector             = "app=conjur-cli"

	// Available templates:
	K8sTemplate         = "secrets-provider-init-container"
	K8sRotationTemplate = "secrets-provider-k8s-rotation"
	P2fTemplate         = "secrets-provider-init-push-to-file"
	P2fRotationTemplate = "secrets-provider-p2f-rotation"
)
