package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

const (
	K8S                        = "k8s_secrets"
	CONJUR_MAP_KEY             = "conjur-map"
	DEFAULT_RETRY_COUNT_LIMIT  = 5
	DEFAULT_RETRY_INTERVAL_SEC = 1
	MIN_RETRY_VALUE            = 0
)

// Config defines the configuration parameters
// for the authentication requests
type Config struct {
	ContainerConfig
	SecretsConfig
}

type ContainerConfig struct {
	PodNamespace       string
	RequiredK8sSecrets []string
	RetryCountLimit    int
	RetryIntervalSec   int
}

type SecretsConfig struct {
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

	retryIntervalSec := parseIntFromEnvOrDefault("RETRY_INTERVAL_SEC", DEFAULT_RETRY_INTERVAL_SEC, MIN_RETRY_VALUE)

	retryCountLimit := parseIntFromEnvOrDefault("RETRY_COUNT_LIMIT", DEFAULT_RETRY_COUNT_LIMIT, MIN_RETRY_VALUE)

	return &Config{
		ContainerConfig: ContainerConfig{
			PodNamespace:       podNamespace,
			RequiredK8sSecrets: requiredK8sSecrets,
			RetryCountLimit:    retryCountLimit,
			RetryIntervalSec:   retryIntervalSec,
		},
		SecretsConfig: SecretsConfig{
			StoreType:          storeType,
		},
	}, nil
}

func parseIntFromEnvOrDefault(environmentVariable string, defaultValue int, minValue int) int {
	envString := os.Getenv(environmentVariable)
	envValueInt, err := strconv.Atoi(envString)
	if err != nil || envValueInt < minValue {
		return defaultValue
	}
	return envValueInt
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
