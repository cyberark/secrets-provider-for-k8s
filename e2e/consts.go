//go:build e2e
// +build e2e

package e2e

const (
	// Namespaces and container names
	SecretsProviderNamespace = "local-secrets-provider"
	ConjurNamespace          = "local-conjur"
	TestAppContainer         = "test-app"
	CLIContainer             = "conjur-cli"

	// Available templates:
	K8sTemplate         = "secrets-provider-init-container"
	K8sRotationTemplate = "secrets-provider-k8s-rotation"
	P2fTemplate         = "secrets-provider-init-push-to-file"
	P2fRotationTemplate = "secrets-provider-p2f-rotation"
)
