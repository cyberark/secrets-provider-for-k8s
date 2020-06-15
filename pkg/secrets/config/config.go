package config

import (
	"os"
	"strings"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

const (
	K8S            = "k8s_secrets"
	CONJUR_MAP_KEY = "conjur-map"
)

// Config defines the configuration parameters
// for the authentication requests
type Config struct {
	PodNamespace       string
	RequiredK8sSecrets []string
	StoreType          string
}

// New returns a new authenticator configuration object
func NewFromEnv() (*Config, error) {

	// Check that required environment variables are set
	for _, envvar := range []string{
		"MY_POD_NAMESPACE",
		"K8S_SECRETS",
		"SECRETS_DESTINATION",
	} {
		if os.Getenv(envvar) == "" {
			return nil, log.RecordedError(messages.CSPFK004E, envvar)
		}
	}

	// Load configuration from the environment
	podNamespace := os.Getenv("MY_POD_NAMESPACE")

	// Remove all white spaces from list
	k8sSecretsList := strings.ReplaceAll(os.Getenv("K8S_SECRETS"), " ", "")
	// Split the comma-separated list into an array
	requiredK8sSecrets := strings.Split(k8sSecretsList, ",")

	storeType := os.Getenv("SECRETS_DESTINATION")
	err := validateStoreType(storeType)
	if err != nil {
		return nil, err
	}

	return &Config{
		PodNamespace:       podNamespace,
		RequiredK8sSecrets: requiredK8sSecrets,
		StoreType:          storeType,
	}, nil
}

func validateStoreType(storeType string) error {
	validStoreTypes := []string{K8S}
	for _, validStoreType := range validStoreTypes {
		if storeType == validStoreType {
			return nil
		}
	}

	return log.RecordedError(messages.CSPFK005E, "SECRETS_DESTINATION")
}
