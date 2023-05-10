package e2e

const (
	SecretsProviderImage = "secrets-provider-for-k8s"
	TestImage            = "debian"
	TestContainer        = "test-app"
	CLIContainer         = "conjur-cli"

	SecretsProviderPodLabels      = "app=test-env"
	SecretsProviderDeploymentName = "test-env"
	SecretsProviderNamespace      = "local-secrets-provider"

	ConjurPodLabels          = "app=conjur-node"
	ConjurServiceName        = "conjur-node"
	ConjurDeploymentName     = "conjur-node"
	ConjurNamespace          = "local-conjur"
	ConjurServiceAccountName = "conjur-cluster"

	ConjurOSSImage          = "cyberark/conjur"
	ConjurOSSImageDest      = "cyberark/conjur:local-conjur"
	ConjurOSSDeploymentName = "conjur-oss"
	ConjurOSSPodLabels      = "app=conjur-oss"

	PostgresImage     = "postgres:10"
	PostgresImageDest = "postgres:local-conjur"

	NginxImageDest = "nginx:local-conjur"

	ConjurCLIPodLabels      = "app=conjur-cli"
	ConjurCLIDeploymentName = "conjur-cli"

	WaitForMaterializedViewRefreshSecond = 2

	// Available templates:
	K8sTemplate         = "secrets-provider-init-container"
	K8sRotationTemplate = "secrets-provider-k8s-rotation"
	P2fTemplate         = "secrets-provider-init-push-to-file"
	P2fRotationTemplate = "secrets-provider-p2f-rotation"
)
