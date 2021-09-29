package pushtofile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type extractSecretGroupsTestCase struct {
	description    string
	input          map[string]string
	expectedOutput SecretGroups
}

const someValidSecrets = "- test/url\n- test-password: test/password\n- test-username: test/username\n"

var extractSecretGroupsTestCases = []extractSecretGroupsTestCase{
	{
		description: "valid annotations map",
		input: map[string]string{
			"conjur.org/authn-identity":                   "host/conjur/authn-k8s/cluster/apps/inventory-api",
			"conjur.org/container-mode":                   "application",
			"conjur.org/secret-destination":               "file",
			"conjur.org/k8s-secret":                       "- k8s-secret-1\n- k8s-secret-2\n",
			"conjur.org/retry-count-limit":                "10",
			"conjur.org/retry-interval-sec":               "5",
			"conjur.org/debug-logging":                    "true",
			"conjur.org/conjur-secrets.cache":             someValidSecrets,
			"conjur.org/secret-file-path.cache":           "this/relative/path",
			"conjur.org/conjur-secrets-policy-path.cache": "some/policy/path",
			"conjur.org/secret-file-template.cache":       "some-template",
			"conjur.org/secret-file-format.cache":         "yaml",
		},
		expectedOutput: SecretGroups{
			{
				Label:                  "cache",
				FilePath:               "this/relative/path",
				FileTemplate:           "some-template",
				ConjurSecretPathPrefix: "some/policy/path",
				SecretSpecs: []SecretSpec{
					{Id: "test/url", Alias: "url"},
					{Id: "test/password", Alias: "test-password"},
					{Id: "test/username", Alias: "test-username"},
				},
				FileFormat: "yaml",
				FilePerms:  defaultFilePerms,
			},
		},
	},
}

func TestNewSecretGroupsFromAnnotations(t *testing.T) {
	for _, tc := range extractSecretGroupsTestCases {
		t.Run(tc.description, func(t *testing.T) {
			resultSecretGroups, err := NewSecretGroupsFromAnnotations(tc.input)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, resultSecretGroups)
		})
	}
}

func TestExtractFileFormatFromAnnotations(t *testing.T) {
	type fileFormatTestCase struct {
		input          string
		expectedOutput string
		valid          bool
	}

	fileFormatTestCases := []fileFormatTestCase{
		{input: "", expectedOutput: "yaml", valid: true},
		{input: "yaml", expectedOutput: "yaml", valid: true},
		{input: "json", expectedOutput: "json", valid: true},
		{input: "dotenv", expectedOutput: "dotenv", valid: true},
		{input: "bash", expectedOutput: "bash", valid: true},
		{input: "plaintext", expectedOutput: "plaintext", valid: true},
		{input: "invalid", expectedOutput: "", valid: false},
	}

	someValidAnnotationsMap := map[string]string{
		"conjur.org/conjur-secrets.cache":   someValidSecrets,
		"conjur.org/secret-file-path.cache": "some/file/name",
	}

	for _, tc := range fileFormatTestCases {
		someValidAnnotationsMap["conjur.org/secret-file-format.cache"] = tc.input

		t.Run("parse file format", func(t *testing.T) {
			secretGroups, err := NewSecretGroupsFromAnnotations(someValidAnnotationsMap)

			if tc.valid {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedOutput, secretGroups[0].FileFormat)
			} else {
				assert.Nil(t, secretGroups)
				assert.Contains(t, err.Error(), "Unknown file format")
			}
		})
	}
}
