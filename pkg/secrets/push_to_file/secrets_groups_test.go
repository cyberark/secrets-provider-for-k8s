package secrets_groups

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type extractSecretsGroupsTestCase struct {
	description string
	contents    map[string]string
	assert      func(t *testing.T, result SecretsGroups, err error)
}

func assertValidSecretsGroups(expected SecretsGroups) func(*testing.T, SecretsGroups, error) {
	return func(t *testing.T, result SecretsGroups, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, len(expected), len(result))
		for expectedKey, expectedVal := range expected {
			resultVal, ok := result[expectedKey]
			assert.True(t, ok)

			assert.Equal(t, expectedVal.Secrets, resultVal.Secrets)
			assert.Equal(t, expectedVal.FilePath, resultVal.FilePath)
			assert.Equal(t, expectedVal.FileType, resultVal.FileType)
			assert.Equal(t, expectedVal.FilePerms, resultVal.FilePerms)

			if expectedVal.FileTemplate != nil {
				assert.NotNil(t, resultVal.FileTemplate)
			}
		}
	}
}

func assertProperError(expectedErr string) func(*testing.T, SecretsGroups, error) {
	return func(t *testing.T, result SecretsGroups, err error) {
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), expectedErr)
	}
}

const someValidSecrets = "- test/url\n- test-password: test/password\n- test-username: test/username\n"
const someValidTemplate = "\"cache\": {\n\t\"url\": {{ .url }},\n\t\"admin-password\": {{ index . \"test-password\" }},\n\t\"admin-username\": {{ index . \"test-username\" }},\n\t\"port\": 123456,\n}"

var extractSecretsGroupsTestCases = []extractSecretsGroupsTestCase{
	{
		description: "valid annotations map",
		contents: map[string]string{
			"conjur.org/authn-identity":                   "host/conjur/authn-k8s/cluster/apps/inventory-api",
			"conjur.org/container-mode":                   "application",
			"conjur.org/secrets-destination":              "file",
			"conjur.org/k8s-secrets":                      "- k8s-secret-1\n- k8s-secret-2\n",
			"conjur.org/retry-count-limit":                "10",
			"conjur.org/retry-interval-sec":               "5",
			"conjur.org/debug-logging":                    "true",
			"conjur.org/conjur-secrets.cache":             someValidSecrets,
			"conjur.org/secret-file-path.cache":           "this/relative/path",
			"conjur.org/conjur-secrets-policy-path.cache": "some/policy/path",
		},
		assert: assertValidSecretsGroups(
			SecretsGroups{
				"cache": {
					Secrets: SecretsPaths{
						"test-password": "/some/policy/path/test/password",
						"test-username": "/some/policy/path/test/username",
						"url":           "/some/policy/path/test/url",
					},
					FilePath:     "/conjur/secrets/this/relative/path",
					FileType:     SecretsFileType(FILE_TYPE_YAML),
					FilePerms:    DEFAULT_FILE_PERMS,
					FileTemplate: nil,
				},
			},
		),
	},
	{
		description: "invalid file path",
		contents: map[string]string{
			"conjur.org/conjur-secrets.cache":   someValidSecrets,
			"conjur.org/secret-file-path.cache": "/this/absolute/path",
		},
		assert: assertProperError("must contain relative path"),
	},
	{
		description: "invalid file path with template specified",
		contents: map[string]string{
			"conjur.org/conjur-secrets.cache":       someValidSecrets,
			"conjur.org/secret-file-path.cache":     "this/relative/directory/",
			"conjur.org/secret-file-template.cache": someValidTemplate,
		},
		assert: assertProperError("template specified but directory found"),
	},
}

func TestExtractSecretsGroupsFromAnnotations(t *testing.T) {
	for _, tc := range extractSecretsGroupsTestCases {
		t.Run(tc.description, func(t *testing.T) {
			secretGroups, err := ExtractSecretsGroupsFromAnnotations(tc.contents)

			tc.assert(t, secretGroups, err)
		})
	}
}

func TestExtractFileTypeFromAnnotations(t *testing.T) {
	type fileTypeTestCase struct {
		input       string
		output      SecretsFileType
		valid       bool
		hasTemplate bool
	}

	fileTypeTestCases := []fileTypeTestCase{
		{input: "", output: SecretsFileType(FILE_TYPE_YAML), valid: true, hasTemplate: false},
		{input: "YAML", output: SecretsFileType(FILE_TYPE_YAML), valid: true, hasTemplate: false},
		{input: "yaml", output: SecretsFileType(FILE_TYPE_YAML), valid: true, hasTemplate: false},
		{input: "JSON", output: SecretsFileType(FILE_TYPE_JSON), valid: true, hasTemplate: false},
		{input: "json", output: SecretsFileType(FILE_TYPE_JSON), valid: true, hasTemplate: false},
		{input: "DOTENV", output: SecretsFileType(FILE_TYPE_DOTENV), valid: true, hasTemplate: false},
		{input: "dotenv", output: SecretsFileType(FILE_TYPE_DOTENV), valid: true, hasTemplate: false},
		{input: "BASH", output: SecretsFileType(FILE_TYPE_BASH), valid: true, hasTemplate: false},
		{input: "bash", output: SecretsFileType(FILE_TYPE_BASH), valid: true, hasTemplate: false},
		{input: "PLAINTEXT", output: SecretsFileType(FILE_TYPE_PLAINTEXT), valid: true, hasTemplate: false},
		{input: "plaintext", output: SecretsFileType(FILE_TYPE_PLAINTEXT), valid: true, hasTemplate: false},
		{input: "invalid", output: SecretsFileType(FILE_TYPE_NONE), valid: false, hasTemplate: false},
		{input: "dotenv", output: SecretsFileType(FILE_TYPE_NONE), valid: true, hasTemplate: true},
		{input: "invalid", output: SecretsFileType(FILE_TYPE_NONE), valid: false, hasTemplate: true},
	}

	secretsGroupTestCase := extractSecretsGroupsTestCase{
		description: "file type annotation",
		contents: map[string]string{
			"conjur.org/conjur-secrets.cache":     someValidSecrets,
			"conjur.org/secret-file-format.cache": "",
			"conjur.org/secret-file-path.cache":   "this/relative/file",
		},
	}

	resultSecretsGroups := SecretsGroups{
		"cache": {
			Secrets: SecretsPaths{
				"test-password": "/test/password",
				"test-username": "/test/username",
				"url":           "/test/url",
			},
			FilePath:     "/conjur/secrets/this/relative/file",
			FilePerms:    DEFAULT_FILE_PERMS,
			FileTemplate: nil,
		},
	}

	for _, tc := range fileTypeTestCases {
		secretsGroupTestCase.contents["conjur.org/secret-file-format.cache"] = tc.input
		group := resultSecretsGroups["cache"]
		group.FileType = tc.output
		resultSecretsGroups["cache"] = group

		if tc.hasTemplate {
			secretsGroupTestCase.contents["conjur.org/secret-file-template.cache"] = someValidTemplate
		} else {
			secretsGroupTestCase.contents["conjur.org/secret-file-template.cache"] = ""
		}

		if tc.valid {
			secretsGroupTestCase.assert = assertValidSecretsGroups(resultSecretsGroups)
		} else {
			secretsGroupTestCase.assert = assertProperError("unknown file format")
		}

		t.Run(secretsGroupTestCase.description, func(t *testing.T) {
			secretGroups, err := ExtractSecretsGroupsFromAnnotations(secretsGroupTestCase.contents)
			secretsGroupTestCase.assert(t, secretGroups, err)
		})
	}
}
