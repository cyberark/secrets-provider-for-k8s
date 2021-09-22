package config

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

func assertEmptyErrorList() func(*testing.T, []error, []error) {
	return func(t *testing.T, errorList []error, infoList []error) {
		assert.Empty(t, errorList)
	}
}

func assertErrorInList(err error) func(*testing.T, []error, []error) {
	return func(t *testing.T, errorList []error, infoList []error) {
		assert.Contains(t, errorList, err)
	}
}

func assertGoodConfig(expected *Config) func(*testing.T, *Config) {
	return func(t *testing.T, result *Config) {
		assert.Equal(t, expected, result)
	}
}

func assertGoodMap(expected map[string]string) func(*testing.T, map[string]string) {
	return func(t *testing.T, result map[string]string) {
		assert.Equal(t, expected, result)
	}
}

func assertInfoInList(err error) func(*testing.T, []error, []error) {
	return func(t *testing.T, errorList []error, infoList []error) {
		assert.Contains(t, infoList, err)
	}
}

type envAndAnnotationsSettingsTestCase struct {
	description string
	annotations map[string]string
	env         map[string]string
	assert      func(t *testing.T, result map[string]string)
}

var envAndAnnotationsSettingsTestCases = []envAndAnnotationsSettingsTestCase{
	{
		description: "the resulting map will be a union of the annotations map and those environment variables pertaining to Secrets Provider config",
		annotations: map[string]string{
			"conjur.org/secrets-destination": "file",
			"conjur.org/container-mode":      "init",
			"unrelated-annotation":           "unrelated",
		},
		env: map[string]string{
			"SECRETS_DESTINATION": "file",
			"RETRY_COUNT_LIMIT":   "5",
			"UNRELATED_ENVVAR":    "UNRELATED",
		},
		assert: assertGoodMap(map[string]string{
			"conjur.org/secrets-destination": "file",
			"conjur.org/container-mode":      "init",
			"unrelated-annotation":           "unrelated",
			"SECRETS_DESTINATION":            "file",
			"RETRY_COUNT_LIMIT":              "5",
		}),
	},
	{
		description: "given an empty annotations map, the returned map should contain the environment",
		annotations: map[string]string{},
		env: map[string]string{
			"MY_POD_NAMESPACE":    "test-namespace",
			"SECRETS_DESTINATION": "file",
			"K8S_SECRETS":         "secret-1,secret-2,secret-3",
			"RETRY_COUNT_LIMIT":   "5",
			"RETRY_INTERVAL_SEC":  "12",
		},
		assert: assertGoodMap(map[string]string{
			"MY_POD_NAMESPACE":    "test-namespace",
			"SECRETS_DESTINATION": "file",
			"K8S_SECRETS":         "secret-1,secret-2,secret-3",
			"RETRY_COUNT_LIMIT":   "5",
			"RETRY_INTERVAL_SEC":  "12",
		}),
	},
	{
		description: "given an empty environment, the returned map should contain the annotations",
		annotations: map[string]string{
			"conjur.org/secrets-destination": "file",
			"conjur.org/container-mode":      "init",
			"unrelated-annotation":           "unrelated",
		},
		env: map[string]string{},
		assert: assertGoodMap(map[string]string{
			"conjur.org/secrets-destination": "file",
			"conjur.org/container-mode":      "init",
			"unrelated-annotation":           "unrelated",
		}),
	},
}

type validSecretsProviderSettingsTestCase struct {
	description  string
	envAndAnnots map[string]string
	assert       func(t *testing.T, errorResults []error, infoResults []error)
}

var validSecretsProviderSettingsTestCases = []validSecretsProviderSettingsTestCase{
	{
		description: "given a valid configuration of annotations, no errors are returned",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"conjur.org/secrets-destination": "file",
			"conjur.org/retry-count-limit":   "10",
			"conjur.org/retry-interval-sec":  "20",
			"conjur.org/k8s-secrets":         `- secret-1\n- secret-2\n- secret-3\n`,
		},
		assert: assertEmptyErrorList(),
	},
	{
		description: "given a valid configuration of envVars, no errors are returned",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":    "test-namespace",
			"SECRETS_DESTINATION": "k8s_secrets",
			"RETRY_COUNT_LIMIT":   "10",
			"RETRY_INTERVAL_SEC":  "20",
			"K8S_SECRETS":         "secret-1,secret-2,secret-3",
		},
		assert: assertEmptyErrorList(),
	},
	{
		description:  "if MY_POD_NAMESPACE envVar is not set, an error is returned",
		envAndAnnots: map[string]string{},
		assert:       assertErrorInList(fmt.Errorf(messages.CSPFK004E, "MY_POD_NAMESPACE")),
	},
	{
		description: "if storeType is not provided by either annotation or envVar, an error is returned",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE": "test-namespace",
		},
		assert: assertErrorInList(errors.New(messages.CSPFK046E)),
	},
	{
		description: "if envVars are used to configure Push-to-File mode, an error is returned",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":    "test-namespace",
			"SECRETS_DESTINATION": "file",
		},
		assert: assertErrorInList(errors.New(messages.CSPFK047E)),
	},
	{
		description: "if a setting is configured with both it's annotation and envVar, an info-level error is returned",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"SECRETS_DESTINATION":            "k8s_secrets",
			"conjur.org/secrets-destination": "file",
		},
		assert: assertInfoInList(fmt.Errorf(messages.CSPFK049E, "StoreType", "SECRETS_DESTINATION", "conjur.org/secrets-destination")),
	},
	{
		description: "if StoreType is configured with an invalid annotation value, and error is returned",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"conjur.org/secrets-destination": "invalid",
		},
		assert: assertErrorInList(fmt.Errorf(messages.CSPFK043E, "conjur.org/secrets-destination", "invalid", []string{"file", "k8s_secrets"})),
	},
	{
		description: "if RequiredK8sSecrets is not configured in K8s Secrets mode, an error is returned",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"conjur.org/secrets-destination": "k8s_secrets",
		},
		assert: assertErrorInList(errors.New(messages.CSPFK048E)),
	},
	{
		description: "an annotation without 'conjur.org/' prefix is results in an info-level error",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"conjur.org/secrets-destination": "file",
			"bad-format":                     "bad-value",
		},
		assert: assertInfoInList(fmt.Errorf(messages.CSPFK011I, "bad-format")),
	},
	{
		description: "an annotation with the 'conjur.org/' prefix that is unrecognized results in an info-level error",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"conjur.org/secrets-destination": "file",
			"conjur.org/bad-annotation":      "bad-value",
		},
		assert: assertInfoInList(fmt.Errorf(messages.CSPFK012I, "conjur.org/bad-annotation")),
	},
	{
		description: "an annotation given a value of an improper type results in an error",
		envAndAnnots: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"conjur.org/secrets-destination": "file",
			"conjur.org/retry-count-limit":   "seven",
		},
		assert: assertErrorInList(fmt.Errorf(messages.CSPFK042E, "conjur.org/retry-count-limit", "seven", "Integer")),
	},
}

type newConfigTestCase struct {
	description string
	settings    map[string]string
	assert      func(t *testing.T, config *Config)
}

var newConfigTestCases = []newConfigTestCase{
	{
		description: "a valid map of annotation-based Secrets Provider settings returns a valid Config",
		settings: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"conjur.org/secrets-destination": "k8s_secrets",
			"conjur.org/k8s-secrets":         `- secret-1\n- secret-2\n- secret-3\n`,
			"conjur.org/retry-count-limit":   "10",
			"conjur.org/retry-interval-sec":  "20",
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       "test-namespace",
			StoreType:          "k8s_secrets",
			RequiredK8sSecrets: []string{"secret-1", "secret-2", "secret-3"},
			RetryCountLimit:    10,
			RetryIntervalSec:   20,
		}),
	},
	{
		description: "a valid map of envVar-based Secrets Provider settings returns a valid Config",
		settings: map[string]string{
			"MY_POD_NAMESPACE":    "test-namespace",
			"SECRETS_DESTINATION": "k8s_secrets",
			"K8S_SECRETS":         "secret-1,secret-2, secret-3",
			"RETRY_COUNT_LIMIT":   "10",
			"RETRY_INTERVAL_SEC":  "20",
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       "test-namespace",
			StoreType:          "k8s_secrets",
			RequiredK8sSecrets: []string{"secret-1", "secret-2", "secret-3"},
			RetryCountLimit:    10,
			RetryIntervalSec:   20,
		}),
	},
	{
		description: "settings configured with both annotations and envVars defer to the annotation value",
		settings: map[string]string{
			"MY_POD_NAMESPACE":               "test-namespace",
			"SECRETS_DESTINATION":            "k8s_secrets",
			"conjur.org/secrets-destination": "file",
			"K8S_SECRETS":                    "secret-1,secret-2,secret-3",
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       "test-namespace",
			StoreType:          "file",
			RequiredK8sSecrets: []string{},
			RetryCountLimit:    DEFAULT_RETRY_COUNT_LIMIT,
			RetryIntervalSec:   DEFAULT_RETRY_INTERVAL_SEC,
		}),
	},
}

func TestEnvAndAnnotationsSettings(t *testing.T) {
	for _, tc := range envAndAnnotationsSettingsTestCases {
		t.Run(tc.description, func(t *testing.T) {
			for envVar, value := range tc.env {
				os.Setenv(envVar, value)
			}

			settingsMap := EnvAndAnnotationSettings(tc.annotations)
			tc.assert(t, settingsMap)

			for envVar := range tc.env {
				os.Unsetenv(envVar)
			}
		})
	}
}

func TestValidSecretsProviderSettings(t *testing.T) {
	for _, tc := range validSecretsProviderSettingsTestCases {
		t.Run(tc.description, func(t *testing.T) {
			errorList, infoList := ValidSecretsProviderSettings(tc.envAndAnnots)
			tc.assert(t, errorList, infoList)
		})
	}
}

func TestNewConfig(t *testing.T) {
	for _, tc := range newConfigTestCases {
		t.Run(tc.description, func(t *testing.T) {
			config := NewConfig(tc.settings)
			tc.assert(t, config)
		})
	}
}
