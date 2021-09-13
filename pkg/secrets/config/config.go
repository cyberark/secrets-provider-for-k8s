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
	FILE                       = "file"
	CONJUR_MAP_KEY             = "conjur-map"
	DEFAULT_RETRY_COUNT_LIMIT  = 5
	DEFAULT_RETRY_INTERVAL_SEC = 1
	MIN_RETRY_VALUE            = 0
)

// Config defines the configuration parameters
// for the authentication requests
type Config struct {
	PodNamespace       string
	RequiredK8sSecrets []string
	RetryCountLimit    int
	RetryIntervalSec   int
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

	retryIntervalSec := parseIntFromStringOrDefault(os.Getenv("RETRY_INTERVAL_SEC"), DEFAULT_RETRY_INTERVAL_SEC, MIN_RETRY_VALUE)

	retryCountLimit := parseIntFromStringOrDefault(os.Getenv("RETRY_COUNT_LIMIT"), DEFAULT_RETRY_COUNT_LIMIT, MIN_RETRY_VALUE)

	return &Config{
		PodNamespace:       podNamespace,
		RequiredK8sSecrets: requiredK8sSecrets,
		RetryCountLimit:    retryCountLimit,
		RetryIntervalSec:   retryIntervalSec,
		StoreType:          storeType,
	}, nil
}

// Returns a new authenticator configuration object, populated from Pod annotations
func NewFromAnnotations(annotations map[string]string) (*Config, error) {

	// check that the only required variable, Secrets Destination, is set
	for _, annotation := range []string{
		"conjur.org/secrets-destination",
	} {
		if _, ok := annotations[annotation]; !ok {
			return nil, log.RecordedError(messages.CSPFK044E, annotation)
		}
	}

	storeType := annotations["conjur.org/secrets-destination"]

	var k8sSecretsArray []string
	var retryIntervalSec int
	var retryCountLimit int

	switch storeType {
	// Default values are only assigned to config fields when in Push-to-File mode.
	// In K8s Secrets mode, these defaults are exchanged for verifiably incorrect values (empty string, -1, nil)
	// so that they can be replaced by values set by the corresponding environment variable.
	case "k8s_secrets":
		// format conjur.org/k8s-secrets as []string, or assign nil if annotation not set
		if k8sSecretsStr, ok := annotations["conjur.org/k8s-secrets"]; ok {
			k8sSecretsStr := strings.ReplaceAll(k8sSecretsStr, "- ", "")
			k8sSecretsArray = strings.Split(k8sSecretsStr, `\n`)
			k8sSecretsArray = k8sSecretsArray[:len(k8sSecretsArray)-1]
		} else {
			k8sSecretsArray = nil
		}

		retryIntervalSec = parseIntFromStringOrDefault(annotations["conjur.org/retry-interval-sec"], -1, MIN_RETRY_VALUE)
		retryCountLimit = parseIntFromStringOrDefault(annotations["conjur.org/retry-count-limit"], -1, MIN_RETRY_VALUE)
	case "file":
		// "conjur.org/k8s-secrets" is ignored when "conjur.org/secrets-destination" is set to "file"
		k8sSecretsArray = []string{}
		retryIntervalSec = parseIntFromStringOrDefault(annotations["conjur.org/retry-interval-sec"], DEFAULT_RETRY_INTERVAL_SEC, MIN_RETRY_VALUE)
		retryCountLimit = parseIntFromStringOrDefault(annotations["conjur.org/retry-count-limit"], DEFAULT_RETRY_COUNT_LIMIT, MIN_RETRY_VALUE)
	}

	// Pod Namespace is still retrieved as an environment variable from the downward API
	return &Config{
		PodNamespace:       "",
		RequiredK8sSecrets: k8sSecretsArray,
		RetryCountLimit:    retryCountLimit,
		RetryIntervalSec:   retryIntervalSec,
		StoreType:          storeType,
	}, nil
}

// Merge updatedConfig into baseConfig
func MergeConfig(baseConfig *Config, updatedConfig *Config) *Config {
	var podNamespace, storeType string
	var requiredK8sSecrets []string
	var retryCountLimit, retryIntervalSec int

	if updatedConfig.PodNamespace != "" {
		podNamespace = updatedConfig.PodNamespace
	} else {
		podNamespace = baseConfig.PodNamespace
	}

	if updatedConfig.RequiredK8sSecrets != nil {
		requiredK8sSecrets = updatedConfig.RequiredK8sSecrets
	} else {
		requiredK8sSecrets = baseConfig.RequiredK8sSecrets
	}

	if updatedConfig.RetryCountLimit != -1 {
		retryCountLimit = updatedConfig.RetryCountLimit
	} else {
		retryCountLimit = baseConfig.RetryCountLimit
	}

	if updatedConfig.RetryIntervalSec != -1 {
		retryIntervalSec = updatedConfig.RetryIntervalSec
	} else {
		retryIntervalSec = baseConfig.RetryIntervalSec
	}

	if updatedConfig.StoreType != "" {
		storeType = updatedConfig.StoreType
	} else {
		storeType = baseConfig.StoreType
	}

	return &Config{
		PodNamespace:       podNamespace,
		RequiredK8sSecrets: requiredK8sSecrets,
		RetryCountLimit:    retryCountLimit,
		RetryIntervalSec:   retryIntervalSec,
		StoreType:          storeType,
	}
}

func parseIntFromStringOrDefault(value string, defaultValue int, minValue int) int {
	valueInt, err := strconv.Atoi(value)
	if err != nil || valueInt < minValue {
		return defaultValue
	}
	return valueInt
}

func validateStoreType(storeType string) error {
	validStoreTypes := []string{K8S, FILE}
	for _, validStoreType := range validStoreTypes {
		if storeType == validStoreType {
			return nil
		}
	}

	return log.RecordedError(messages.CSPFK005E, "SECRETS_DESTINATION")
}
