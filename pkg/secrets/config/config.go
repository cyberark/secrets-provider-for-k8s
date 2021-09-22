package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

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

// Define supported annotation keys for Secrets Provider config, as well as value restraints for each
var secretsProviderAnnotations = map[string][]string{
	"conjur.org/authn-identity":      {"string"},
	"conjur.org/container-mode":      {"string", "init", "application"},
	"conjur.org/secrets-destination": {"string", "file", "k8s_secrets"},
	"conjur.org/k8s-secrets":         {"string"},
	"conjur.org/retry-count-limit":   {"int"},
	"conjur.org/retry-interval-sec":  {"int"},
	"conjur.org/debug-logging":       {"bool"},
}

// Define supported annotation key prefixes for Push to File config, as well as value restraints for each.
// In use, Push to File keys include a secret group ("conjur.org/conjur-secrets.{secret-group}").
// The values listed here will confirm hardcoded formatting, dynamic annotation content will
// be validated when used.
var pushToFileAnnotationPrefixes = map[string][]string{
	"conjur.org/conjur-secrets.":             {"string"},
	"conjur.org/conjur-secrets-policy-path.": {"string"},
	"conjur.org/secret-file-path.":           {"string"},
	"conjur.org/secret-file-format.":         {"string", "yaml", "json", "dotenv", "bash"},
	"conjur.org/secret-file-template.":       {"string"},
}

// Define environment variables used in Secrets Provider config
var validEnvVars = []string{
	"MY_POD_NAMESPACE",
	"SECRETS_DESTINATION",
	"K8S_SECRETS",
	"RETRY_INTERVAL_SEC",
	"RETRY_COUNT_LIMIT",
}

func EnvAndAnnotationSettings(annotations map[string]string) map[string]string {
	// Return a master map of supplied Secrets Provider Config settings
	// Returned map contains envVar settings and all annotations

	masterMap := make(map[string]string)

	for annotation, value := range annotations {
		masterMap[annotation] = value
	}

	for _, envVar := range validEnvVars {
		value := os.Getenv(envVar)
		if value != "" {
			masterMap[envVar] = value
		}
	}

	return masterMap
}

func ValidSecretsProviderSettings(envAndAnnots map[string]string) ([]error, []error) {
	// Validate that required environment variables exist
	// Returns two lists of errors: one of Error level, and one of Info level

	errorList := []error{}
	infoList := []error{}

	if envAndAnnots["MY_POD_NAMESPACE"] == "" {
		errorList = append(errorList, fmt.Errorf(messages.CSPFK004E, "MY_POD_NAMESPACE"))
	}

	envStoreType := envAndAnnots["SECRETS_DESTINATION"]
	annotStoreType := envAndAnnots["conjur.org/secrets-destination"]
	storeType := ""

	if annotStoreType == "" {
		switch envStoreType {
		case "":
			errorList = append(errorList, errors.New(messages.CSPFK046E))
		case FILE:
			errorList = append(errorList, errors.New(messages.CSPFK047E))
		case K8S:
			storeType = envStoreType
		}
	} else if validStoreType(annotStoreType) {
		if validStoreType(envStoreType) {
			infoList = append(infoList, fmt.Errorf(messages.CSPFK049E, "StoreType", "SECRETS_DESTINATION", "conjur.org/secrets-destination"))
		}
		storeType = annotStoreType
	} else {
		annotError := fmt.Errorf(messages.CSPFK043E, "conjur.org/secrets-destination", annotStoreType, []string{FILE, K8S})
		if validStoreType(envStoreType) {
			storeType = envStoreType
			infoList = append(infoList, annotError)
		} else if envStoreType != "" {
			errorList = append(errorList, annotError)
			errorList = append(errorList, fmt.Errorf(messages.CSPFK005E, "SECRETS_DESTINATION"))
		}
	}

	envK8sSecretsStr := envAndAnnots["K8S_SECRETS"]
	annotK8sSecretsStr := envAndAnnots["conjur.org/k8s-secrets"]
	if storeType == "k8s_secrets" {
		if envK8sSecretsStr == "" && annotK8sSecretsStr == "" {
			errorList = append(errorList, errors.New(messages.CSPFK048E))
		} else if envK8sSecretsStr != "" && annotK8sSecretsStr != "" {
			infoList = append(infoList, fmt.Errorf(messages.CSPFK049E, "RequiredK8sSecrets", "K8S_SECRETS", "conjur.org/k8s-secrets"))
		}
	}

	annotRetryCountLimit := envAndAnnots["conjur.org/retry-count-limit"]
	envRetryCountLimit := envAndAnnots["RETRY_COUNT_LIMIT"]
	if annotRetryCountLimit != "" && envRetryCountLimit != "" {
		infoList = append(infoList, fmt.Errorf(messages.CSPFK049E, "RetryCountLimit", "RETRY_COUNT_LIMIT", "conjur.org/retry-count-limit"))
	}

	annotRetryIntervalSec := envAndAnnots["conjur.org/retry-interval-sec"]
	envRetryIntervalSec := envAndAnnots["RETRY_INTERVAL_SEC"]
	if annotRetryIntervalSec != "" && envRetryIntervalSec != "" {
		infoList = append(infoList, fmt.Errorf(messages.CSPFK049E, "RetryIntervalSec", "RETRY_INTERVAL_SEC", "conjur.org/retry-interval-sec"))
	}

	for setting, value := range envAndAnnots {
		if !valueInArray(setting, validEnvVars) {
			if match, foundMap, err := validateAnnotationKey(setting); err == nil {
				acceptedValueInfo := foundMap[match]
				err := validateAnnotationValue(setting, value, acceptedValueInfo)
				if err != nil {
					errorList = append(errorList, err)
				}
			} else {
				infoList = append(infoList, err)
			}
		}
	}

	return errorList, infoList
}

func NewConfig(settings map[string]string) *Config {
	// Create a new config from environment variable and annotation settings
	// Provided annotations are already validated in format and value

	podNamespace := settings["MY_POD_NAMESPACE"]

	storeType := settings["conjur.org/secrets-destination"]
	if storeType == "" {
		storeType = settings["SECRETS_DESTINATION"]
	}

	k8sSecretsArr := []string{}
	if storeType != "file" {
		k8sSecretsStr := settings["conjur.org/k8s-secrets"]
		if k8sSecretsStr != "" {
			k8sSecretsStr := strings.ReplaceAll(k8sSecretsStr, "- ", "")
			k8sSecretsArr = strings.Split(k8sSecretsStr, "\n")
			k8sSecretsArr = k8sSecretsArr[:len(k8sSecretsArr)-1]
		} else {
			k8sSecretsStr = settings["K8S_SECRETS"]
			k8sSecretsStr = strings.ReplaceAll(k8sSecretsStr, " ", "")
			k8sSecretsArr = strings.Split(k8sSecretsStr, ",")
		}
	}

	retryCountLimitStr := settings["conjur.org/retry-count-limit"]
	if retryCountLimitStr == "" {
		retryCountLimitStr = settings["RETRY_COUNT_LIMIT"]
	}
	retryCountLimit := parseIntFromStringOrDefault(retryCountLimitStr, DEFAULT_RETRY_COUNT_LIMIT, MIN_RETRY_VALUE)

	retryIntervalSecStr := settings["conjur.org/retry-interval-sec"]
	if retryIntervalSecStr == "" {
		retryIntervalSecStr = settings["RETRY_INTERVAL_SEC"]
	}
	retryIntervalSec := parseIntFromStringOrDefault(retryIntervalSecStr, DEFAULT_RETRY_INTERVAL_SEC, MIN_RETRY_VALUE)

	return &Config{
		PodNamespace:       podNamespace,
		RequiredK8sSecrets: k8sSecretsArr,
		RetryCountLimit:    retryCountLimit,
		RetryIntervalSec:   retryIntervalSec,
		StoreType:          storeType,
	}
}

func validateAnnotationKey(key string) (string, map[string][]string, error) {
	// Validate that a given annotation key is formatted as "conjur.org/xyz"
	// Record Info level log if a key conforms to the formatting standard but
	// is not recognized as either a Secrets Provider config or Push to File config annotation
	//
	// If the annotation is for Push to File config, the ValidAnnotations function
	// needs to be aware of the annotation's valid prefix in order to perform input validation,
	// so this function returns:
	//   - either the key, or the key's valid prefix
	//   - the Map in which the key or prefix was found
	//   - the success status of the operation
	if !strings.HasPrefix(key, "conjur.org/") {
		return "", nil, fmt.Errorf(messages.CSPFK011I, key)
	}

	if valueInMapKeys(key, secretsProviderAnnotations) {
		return key, secretsProviderAnnotations, nil
	} else if prefix, ok := valuePrefixInMapKeys(key, pushToFileAnnotationPrefixes); ok {
		return prefix, pushToFileAnnotationPrefixes, nil
	} else {
		return "", nil, fmt.Errorf(messages.CSPFK012I, key)
	}
}

func validateAnnotationValue(key string, value string, acceptedValueInfo []string) error {
	// given a key/value pair, where the key is confirmed to be a Secrets Provider config annotation
	// validated that the value is of valid type, or confirm that the value is
	// in the range of enumerated and acceptable values
	switch targetType := acceptedValueInfo[0]; targetType {
	case "int":
		_, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf(messages.CSPFK042E, key, value, "Integer")
		}
	case "bool":
		if value != "true" && value != "false" {
			return fmt.Errorf(messages.CSPFK042E, key, value, "Boolean")
		}
	case "string":
		acceptedValues := acceptedValueInfo[1:]
		if len(acceptedValues) > 0 && !valueInArray(value, acceptedValues) {
			return fmt.Errorf(messages.CSPFK043E, key, value, acceptedValues)
		}
	}
	return nil
}

func valueInMapKeys(value string, searchMap map[string][]string) bool {
	if _, ok := searchMap[value]; ok {
		return true
	} else {
		return false
	}
}

func valuePrefixInMapKeys(value string, searchMap map[string][]string) (string, bool) {
	for key := range searchMap {
		if strings.HasPrefix(value, key) {
			return key, true
		}
	}
	return "", false
}

func valueInArray(value string, array []string) bool {
	for _, item := range array {
		if value == item {
			return true
		}
	}
	return false
}

func parseIntFromStringOrDefault(value string, defaultValue int, minValue int) int {
	valueInt, err := strconv.Atoi(value)
	if err != nil || valueInt < minValue {
		return defaultValue
	}
	return valueInt
}

func validStoreType(storeType string) bool {
	validStoreTypes := []string{K8S, FILE}
	for _, validStoreType := range validStoreTypes {
		if storeType == validStoreType {
			return true
		}
	}
	return false
}
