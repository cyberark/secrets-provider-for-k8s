package pushtofile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type assertFunc func(*testing.T, SecretGroups, error)

type extractSecretGroupsTestCase struct {
	description string
	contents    map[string]string
	assert      assertFunc
}

func assertExpectedSecretGroups(expected SecretGroups) assertFunc {
	return func(t *testing.T, result SecretGroups, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, expected, result)
	}
}

const someValidSecrets = "- test/url\n- test-password: test/password\n- test-username: test/username\n"

var extractSecretGroupsTestCases = []extractSecretGroupsTestCase{
	{
		description: "valid annotations map",
		contents: map[string]string{
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
		assert: assertExpectedSecretGroups(
			SecretGroups{
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
		),
	},
	{
		description: "valid annotations map",
		contents: map[string]string{
			"conjur.org/conjur-secrets.cache":             someValidSecrets,
			"conjur.org/secret-file-path.cache":           "this/relative/path",
			"conjur.org/conjur-secrets-policy-path.cache": "some/policy/path",
			"conjur.org/secret-file-template.cache":       "some-template",
			"conjur.org/secret-file-format.cache":         "yaml",
		},
		assert: assertExpectedSecretGroups(
			SecretGroups{
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
		),
	},
}

func TestNewSecretGroupsFromAnnotations(t *testing.T) {
	for _, tc := range extractSecretGroupsTestCases {
		t.Run(tc.description, func(t *testing.T) {
			secretGroups, err := NewSecretGroupsFromAnnotations(tc.contents)

			tc.assert(t, secretGroups, err)
		})
	}
}

func TestExtractFileFormatFromAnnotations(t *testing.T) {
	type fileFormatTestCase struct {
		input  string
		output string
		valid  bool
	}

	fileFormatTestCases := []fileFormatTestCase{
		{input: "", output: "yaml", valid: true},
		{input: "yaml", output: "yaml", valid: true},
		{input: "json", output: "json", valid: true},
		{input: "dotenv", output: "dotenv", valid: true},
		{input: "bash", output: "bash", valid: true},
		{input: "plaintext", output: "plaintext", valid: true},
		{input: "invalid", output: "", valid: false},
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
				assert.Equal(t, tc.output, secretGroups[0].FileFormat)
			} else {
				assert.Nil(t, secretGroups)
				assert.Contains(t, err.Error(), "Unknown file format")
			}
		})
	}
}
