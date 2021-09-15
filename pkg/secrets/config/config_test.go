package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type newFromSourceTestCase struct {
	description string
	contents    map[string]string
	assert      func(t *testing.T, result *Config, err error)
}

type mergeConfigTestCase struct {
	description string
	contents    []*Config
	assert      func(t *testing.T, result *Config, err error)
}

func assertGoodConfig(expected *Config) func(*testing.T, *Config, error) {
	return func(t *testing.T, result *Config, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, expected, result)
	}
}

func assertProperError(expectedErr string) func(*testing.T, *Config, error) {
	return func(t *testing.T, result *Config, err error) {
		assert.Contains(t, err.Error(), expectedErr)
	}
}

var namespace = "test-namespace"
var destination = "k8s_secrets"
var k8sSecretsStr = "test-k8s-secret,other-k8s-secret, k8s-secret-after-a-space"
var k8sSecretsList = strings.Split(strings.ReplaceAll(k8sSecretsStr, " ", ""), ",")

var newFromEnvTestCases = []newFromSourceTestCase{
	{
		description: "valid environment variable settings",
		contents: map[string]string{
			"MY_POD_NAMESPACE":    namespace,
			"SECRETS_DESTINATION": destination,
			"K8S_SECRETS":         k8sSecretsStr,
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       namespace,
			RequiredK8sSecrets: k8sSecretsList,
			RetryCountLimit:    5,
			RetryIntervalSec:   1,
			StoreType:          destination,
		}),
	},
	{
		description: "missing MY_POD_NAMESPACE environment variable",
		contents: map[string]string{
			"K8S_SECRETS":         k8sSecretsStr,
			"SECRETS_DESTINATION": destination,
		},
		assert: assertProperError("Environment variable 'MY_POD_NAMESPACE' must be provided"),
	},
	{
		description: "missing K8S_SECRETS environment variable",
		contents: map[string]string{
			"MY_POD_NAMESPACE":    namespace,
			"SECRETS-DESTINATION": destination,
		},
		assert: assertProperError("Environment variable 'K8S_SECRETS' must be provided"),
	},
	{
		description: "missing SECRETS_DESTINATION environment variable",
		contents: map[string]string{
			"MY_POD_NAMESPACE": namespace,
			"K8S_SECRETS":      k8sSecretsStr,
		},
		assert: assertProperError("Environment variable 'SECRETS_DESTINATION' must be provided"),
	},
	{
		description: "invalid value provided for SECRETS_DESTINATION",
		contents: map[string]string{
			"MY_POD_NAMESPACE":    namespace,
			"SECRETS_DESTINATION": "bad-destination",
			"K8S_SECRETS":         k8sSecretsStr,
		},
		assert: assertProperError("Provided incorrect value for environment variable"),
	},
}

var newFromAnnotationsTestCases = []newFromSourceTestCase{
	{
		description: "conjur.org/secrets-destination not set",
		contents: map[string]string{
			"conjur.org/k8s-secrets":        `- k8s-secret-1\n- k8s-secret-2\n`,
			"conjur.org/retry-count-limit":  "10",
			"conjur.org/retry-interval-sec": "5",
		},
		assert: assertProperError("Annotation 'conjur.org/secrets-destination' must be provided"),
	},
	{
		description: "in Push-to-File mode, annotations not provided will be set to defaults",
		contents: map[string]string{
			"conjur.org/secrets-destination": "file",
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       "",
			RequiredK8sSecrets: []string{},
			RetryCountLimit:    5,
			RetryIntervalSec:   1,
			StoreType:          "file",
		}),
	},
	{
		description: "in Push-to-File mode, provided annotations are in the returned Config",
		contents: map[string]string{
			"conjur.org/secrets-destination": "file",
			"conjur.org/k8s-secrets":         `- k8s-secret-1\n- k8s-secret-2\n`,
			"conjur.org/retry-count-limit":   "10",
			"conjur.org/retry-interval-sec":  "5",
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       "",
			RequiredK8sSecrets: []string{},
			RetryCountLimit:    10,
			RetryIntervalSec:   5,
			StoreType:          "file",
		}),
	},
	{
		description: "in K8s Secrets mode, annotations not provided indicate as such",
		contents: map[string]string{
			"conjur.org/secrets-destination": "k8s_secrets",
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       "",
			RequiredK8sSecrets: nil,
			RetryCountLimit:    -1,
			RetryIntervalSec:   -1,
			StoreType:          "k8s_secrets",
		}),
	},
	{
		description: "in K8s Secrets mode, provided annotations are in the returned Config",
		contents: map[string]string{
			"conjur.org/secrets-destination": "k8s_secrets",
			"conjur.org/k8s-secrets":         `- k8s-secret-1\n- k8s-secret-2\n`,
			"conjur.org/retry-count-limit":   "10",
			"conjur.org/retry-interval-sec":  "5",
		},
		assert: assertGoodConfig(&Config{
			PodNamespace:       "",
			RequiredK8sSecrets: []string{"k8s-secret-1", "k8s-secret-2"},
			RetryCountLimit:    10,
			RetryIntervalSec:   5,
			StoreType:          "k8s_secrets",
		}),
	},
}

var envConfig = &Config{
	PodNamespace:       "env-test-namespace",
	RequiredK8sSecrets: []string{"test-k8s-secret", "other-k8s-secret", "k8s-secret-after-a-space"},
	RetryCountLimit:    10,
	RetryIntervalSec:   10,
	StoreType:          "k8s_secrets",
}

var goodAnnotConfig = &Config{
	PodNamespace:       "annot-test-namespace",
	RequiredK8sSecrets: []string{},
	RetryCountLimit:    20,
	RetryIntervalSec:   20,
	StoreType:          "file",
}

var badAnnotConfig = &Config{
	PodNamespace:       "",
	RequiredK8sSecrets: nil,
	RetryCountLimit:    -1,
	RetryIntervalSec:   -1,
	StoreType:          "",
}

var mergeConfigTestCases = []mergeConfigTestCase{
	{
		description: "given two fully-configured Configs",
		contents:    []*Config{envConfig, goodAnnotConfig},
		assert:      assertGoodConfig(goodAnnotConfig),
	},
	{
		description: "given a second Config that indicates no annotation was provided in K8s Secret mode",
		contents:    []*Config{envConfig, badAnnotConfig},
		assert:      assertGoodConfig(envConfig),
	},
}

func TestNewFromEnv(t *testing.T) {
	for _, tc := range newFromEnvTestCases {
		t.Run(tc.description, func(t *testing.T) {
			for envvar, value := range tc.contents {
				_ = os.Setenv(envvar, value)
			}

			config, err := NewFromEnv()
			tc.assert(t, config, err)

			for envvar := range tc.contents {
				_ = os.Unsetenv(envvar)
			}
		})
	}
}

func TestNewFromAnnotations(t *testing.T) {
	for _, tc := range newFromAnnotationsTestCases {
		t.Run(tc.description, func(t *testing.T) {
			config, err := NewFromAnnotations(tc.contents)
			tc.assert(t, config, err)
		})
	}
}

func TestMergeConfig(t *testing.T) {
	for _, tc := range mergeConfigTestCases {
		t.Run(tc.description, func(t *testing.T) {
			config := MergeConfig(tc.contents[0], tc.contents[1])
			tc.assert(t, config, nil)
		})
	}
}
