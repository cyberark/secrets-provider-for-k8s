//go:build e2e
// +build e2e

package e2e

const (
	// Container names
	TestAppContainer       = "test-app"
	ConjurClusterContainer = "conjur-appliance"
	ConjurCLIContainer     = "conjur-cli"
	LogsContainer          = "cyberark-secrets-provider-for-k8s"

	// Label selectors
	SPLabelSelector             = "app=test-env"
	SPHelmLabelSelector         = "app=test-helm"
	ConjurCLILabelSelector      = "app=conjur-cli"
	ConjurFollowerLabelSelector = "role=follower"

	// Available templates
	K8sTemplate         = "secrets-provider-init-container"
	K8sRotationTemplate = "secrets-provider-k8s-rotation"
	P2fTemplate         = "secrets-provider-init-push-to-file"
	P2fRotationTemplate = "secrets-provider-p2f-rotation"
)
