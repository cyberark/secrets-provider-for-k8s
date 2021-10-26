package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// Constants for Secrets Provider operation modes,
// and Defaults for some SP settings
const (
	K8s                     = "k8s_secrets"
	File                    = "file"
	ConjurMapKey            = "conjur-map"
	DefaultRetryCountLimit  = 5
	DefaultRetryIntervalSec = 1
	MinRetryValue           = 0
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

type annotationType int

// Represents each annotation input value type,
// used during input value validation
const (
	TYPESTRING annotationType = iota
	TYPEINT
	TYPEBOOL
)

type annotationRestraints struct {
	allowedType   annotationType
	allowedValues []string
}

// Define supported annotation keys for Secrets Provider config, as well as value restraints for each
var secretsProviderAnnotations = map[string]annotationRestraints{
	"conjur.org/authn-identity":      {TYPESTRING, []string{}},
	"conjur.org/container-mode":      {TYPESTRING, []string{"init", "application"}},
	"conjur.org/secrets-destination": {TYPESTRING, []string{"file", "k8s_secrets"}},
	"conjur.org/k8s-secrets":         {TYPESTRING, []string{}},
	"conjur.org/retry-count-limit":   {TYPEINT, []string{}},
	"conjur.org/retry-interval-sec":  {TYPEINT, []string{}},
	"conjur.org/debug-logging":       {TYPEBOOL, []string{}},
}

// Define supported annotation key prefixes for Push to File config, as well as value restraints for each.
// In use, Push to File keys include a secret group ("conjur.org/conjur-secrets.{secret-group}").
// The values listed here will confirm hardcoded formatting, dynamic annotation content will
// be validated when used.
var pushToFileAnnotationPrefixes = map[string]annotationRestraints{
	"conjur.org/conjur-secrets.":             {TYPESTRING, []string{}},
	"conjur.org/conjur-secrets-policy-path.": {TYPESTRING, []string{}},
	"conjur.org/secret-file-path.":           {TYPESTRING, []string{}},
	"conjur.org/secret-file-format.":         {TYPESTRING, []string{"yaml", "json", "dotenv", "bash"}},
	"conjur.org/secret-file-template":        {TYPESTRING, []string{}},
}

// Define environment variables used in Secrets Provider config
var validEnvVars = []string{
	"MY_POD_NAMESPACE",
	"SECRETS_DESTINATION",
	"K8S_SECRETS",
	"RETRY_INTERVAL_SEC",
	"RETRY_COUNT_LIMIT",
}

// ValidateAnnotations confirms that the provided annotations are properly
// formated, have the proper value type, and if the annotation in question
// had a defined set of accepted values, the provided value is confirmed.
// Function returns a list of Error logs, and a list of Info logs.
func ValidateAnnotations(annotations map[string]string) ([]error, []error) {
	errorList := []error{}
	infoList := []error{}

	for key, value := range annotations {
		if match, foundMap, err := validateAnnotationKey(key); err == nil {
			acceptedValueInfo := foundMap[match]
			err := validateAnnotationValue(key, value, acceptedValueInfo)
			if err != nil {
				errorList = append(errorList, err)
			}
		} else {
			infoList = append(infoList, err)
		}
	}

	return errorList, infoList
}

// GatherSecretsProviderSettings returns a string-to-string map of all provided environment
// variables and parsed, valid annotations that are concerned with Secrets Provider Config.
func GatherSecretsProviderSettings(annotations map[string]string) map[string]string {
	masterMap := make(map[string]string)

	for annotation, value := range annotations {
		if _, ok := secretsProviderAnnotations[annotation]; ok {
			masterMap[annotation] = value
		}
	}

	for _, envVar := range validEnvVars {
		value := os.Getenv(envVar)
		if value != "" {
			masterMap[envVar] = value
		}
	}

	return masterMap
}

// ValidateSecretsProviderSettings confirms that the provided environment variable and annotation
// settings yield a valid Secrets Provider configuration. Returns a list of Error logs, and a list
// of Info logs.
func ValidateSecretsProviderSettings(envAndAnnots map[string]string) ([]error, []error) {
	var errorList []error
	var infoList []error

	// PodNamespace must be configured by envVar
	if envAndAnnots["MY_POD_NAMESPACE"] == "" {
		errorList = append(errorList, fmt.Errorf(messages.CSPFK004E, "MY_POD_NAMESPACE"))
	}

	envStoreType := envAndAnnots["SECRETS_DESTINATION"]
	annotStoreType := envAndAnnots["conjur.org/secrets-destination"]
	storeType := ""

	if annotStoreType == "" {
		switch envStoreType {
		case K8s:
			storeType = envStoreType
		case File:
			errorList = append(errorList, errors.New(messages.CSPFK047E))
		case "":
			errorList = append(errorList, errors.New(messages.CSPFK046E))
		default:
			errorList = append(errorList, fmt.Errorf(messages.CSPFK005E, "SECRETS_DESTINATION"))
		}
	} else if validStoreType(annotStoreType) {
		if validStoreType(envStoreType) {
			infoList = append(infoList, fmt.Errorf(messages.CSPFK012I, "StoreType", "SECRETS_DESTINATION", "conjur.org/secrets-destination"))
		}
		storeType = annotStoreType
	} else {
		errorList = append(errorList, fmt.Errorf(messages.CSPFK043E, "conjur.org/secrets-destination", annotStoreType, []string{File, K8s}))
	}

	envK8sSecretsStr := envAndAnnots["K8S_SECRETS"]
	annotK8sSecretsStr := envAndAnnots["conjur.org/k8s-secrets"]
	if storeType == "k8s_secrets" {
		if envK8sSecretsStr == "" && annotK8sSecretsStr == "" {
			errorList = append(errorList, errors.New(messages.CSPFK048E))
		} else if envK8sSecretsStr != "" && annotK8sSecretsStr != "" {
			infoList = append(infoList, fmt.Errorf(messages.CSPFK012I, "RequiredK8sSecrets", "K8S_SECRETS", "conjur.org/k8s-secrets"))
		}
	}

	annotRetryCountLimit := envAndAnnots["conjur.org/retry-count-limit"]
	envRetryCountLimit := envAndAnnots["RETRY_COUNT_LIMIT"]
	if annotRetryCountLimit != "" && envRetryCountLimit != "" {
		infoList = append(infoList, fmt.Errorf(messages.CSPFK012I, "RetryCountLimit", "RETRY_COUNT_LIMIT", "conjur.org/retry-count-limit"))
	}

	annotRetryIntervalSec := envAndAnnots["conjur.org/retry-interval-sec"]
	envRetryIntervalSec := envAndAnnots["RETRY_INTERVAL_SEC"]
	if annotRetryIntervalSec != "" && envRetryIntervalSec != "" {
		infoList = append(infoList, fmt.Errorf(messages.CSPFK012I, "RetryIntervalSec", "RETRY_INTERVAL_SEC", "conjur.org/retry-interval-sec"))
	}

	return errorList, infoList
}

// NewConfig creates a new Secrets Provider configuration for a validated
// map of environment variable and annotation settings.
func NewConfig(settings map[string]string) *Config {
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
	retryCountLimit := parseIntFromStringOrDefault(retryCountLimitStr, DefaultRetryCountLimit, MinRetryValue)

	retryIntervalSecStr := settings["conjur.org/retry-interval-sec"]
	if retryIntervalSecStr == "" {
		retryIntervalSecStr = settings["RETRY_INTERVAL_SEC"]
	}
	retryIntervalSec := parseIntFromStringOrDefault(retryIntervalSecStr, DefaultRetryIntervalSec, MinRetryValue)

	return &Config{
		PodNamespace:       podNamespace,
		RequiredK8sSecrets: k8sSecretsArr,
		RetryCountLimit:    retryCountLimit,
		RetryIntervalSec:   retryIntervalSec,
		StoreType:          storeType,
	}
}

// If the annotation being validated is for Push to File config, the ValidAnnotations function
// needs to be aware of the annotation's valid prefix in order to perform input validation,
// so this function returns:
//   - either the key, or the key's valid prefix
//   - the Map in which the key or prefix was found
//   - the success status of the operation
func validateAnnotationKey(key string) (string, map[string]annotationRestraints, error) {
	if strings.HasPrefix(key, "conjur.org/") {
		if _, ok := secretsProviderAnnotations[key]; ok {
			return key, secretsProviderAnnotations, nil
		} else if prefix, ok := valuePrefixInMapKeys(key, pushToFileAnnotationPrefixes); ok {
			return prefix, pushToFileAnnotationPrefixes, nil
		} else {
			return "", nil, fmt.Errorf(messages.CSPFK011I, key)
		}
	}
	return "", nil, nil
}

func validateAnnotationValue(key string, value string, acceptedValueInfo annotationRestraints) error {
	switch targetType := acceptedValueInfo.allowedType; targetType {
	case TYPEINT:
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf(messages.CSPFK042E, key, value, "Integer")
		}
	case TYPEBOOL:
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf(messages.CSPFK042E, key, value, "Boolean")
		}
	case TYPESTRING:
		acceptedValues := acceptedValueInfo.allowedValues
		if len(acceptedValues) > 0 && !valueInArray(value, acceptedValues) {
			return fmt.Errorf(messages.CSPFK043E, key, value, acceptedValues)
		}
	}
	return nil
}

func valuePrefixInMapKeys(value string, searchMap map[string]annotationRestraints) (string, bool) {
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
	validStoreTypes := []string{K8s, File}
	for _, validStoreType := range validStoreTypes {
		if storeType == validStoreType {
			return true
		}
	}
	return false
}
