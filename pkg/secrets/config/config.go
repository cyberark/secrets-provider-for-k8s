package config

import (
	"os"
	"strings"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
)

// Config defines the configuration parameters
// for the authentication requests
type Config struct {
	PodNamespace       string
	RequiredK8sSecrets []string
}

const CONJUR_MAP_KEY = "conjur-map"

// New returns a new authenticator configuration object
func NewFromEnv() (*Config, error) {

	// Check that required environment variables are set
	for _, envvar := range []string{
		"MY_POD_NAMESPACE",
		"K8S_SECRETS",
	} {
		if os.Getenv(envvar) == "" {
			return nil, log.RecorderError(log.CSPFK017E, envvar)
		}
	}

	// Load configuration from the environment
	podNamespace := os.Getenv("MY_POD_NAMESPACE")

	// Split the comma-separated list into an array
	requiredK8sSecrets := strings.Split(os.Getenv("K8S_SECRETS"), ",")

	return &Config{
		PodNamespace:       podNamespace,
		RequiredK8sSecrets: requiredK8sSecrets,
	}, nil
}
