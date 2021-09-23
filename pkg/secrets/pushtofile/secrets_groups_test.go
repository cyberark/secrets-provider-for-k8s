package pushtofile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type assertFunc func(*testing.T, SecretsGroups, error)

type extractSecretsGroupsTestCase struct {
	description string
	contents    map[string]string
	assert      assertFunc
}

func assertExpectedSecretsGroups(expected SecretsGroups) assertFunc {
	return func(t *testing.T, result SecretsGroups, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, expected, result)
	}
}

func assertProperError(expectedErr string) assertFunc {
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
		assert: assertExpectedSecretsGroups(
			SecretsGroups{
				"cache": {
					Secrets: SecretsPaths{
						"test-password": "/some/policy/path/test/password",
						"test-username": "/some/policy/path/test/username",
						"url":           "/some/policy/path/test/url",
					},
					FilePath:     "/conjur/secrets/this/relative/path",
					FileFormat:   SecretsFileFormat(FILE_FORMAT_YAML),
					FilePerms:    DEFAULT_FILE_PERMS,
					FileTemplate: "",
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
		assert: assertProperError("contains directory instead of file"),
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

func TestExtractFileFormatFromAnnotations(t *testing.T) {
	type fileFormatTestCase struct {
		input       string
		output      SecretsFileFormat
		valid       bool
		hasTemplate bool
	}

	fileFormatTestCases := []fileFormatTestCase{
		{input: "", output: SecretsFileFormat(FILE_FORMAT_YAML), valid: true, hasTemplate: false},
		{input: "YAML", output: SecretsFileFormat(FILE_FORMAT_YAML), valid: true, hasTemplate: false},
		{input: "yaml", output: SecretsFileFormat(FILE_FORMAT_YAML), valid: true, hasTemplate: false},
		{input: "JSON", output: SecretsFileFormat(FILE_FORMAT_JSON), valid: true, hasTemplate: false},
		{input: "json", output: SecretsFileFormat(FILE_FORMAT_JSON), valid: true, hasTemplate: false},
		{input: "DOTENV", output: SecretsFileFormat(FILE_FORMAT_DOTENV), valid: true, hasTemplate: false},
		{input: "dotenv", output: SecretsFileFormat(FILE_FORMAT_DOTENV), valid: true, hasTemplate: false},
		{input: "BASH", output: SecretsFileFormat(FILE_FORMAT_BASH), valid: true, hasTemplate: false},
		{input: "bash", output: SecretsFileFormat(FILE_FORMAT_BASH), valid: true, hasTemplate: false},
		{input: "PLAINTEXT", output: SecretsFileFormat(FILE_FORMAT_PLAINTEXT), valid: true, hasTemplate: false},
		{input: "plaintext", output: SecretsFileFormat(FILE_FORMAT_PLAINTEXT), valid: true, hasTemplate: false},
		{input: "invalid", output: SecretsFileFormat(FILE_FORMAT_NONE), valid: false, hasTemplate: false},
		{input: "dotenv", output: SecretsFileFormat(FILE_FORMAT_NONE), valid: true, hasTemplate: true},
		{input: "invalid", output: SecretsFileFormat(FILE_FORMAT_NONE), valid: false, hasTemplate: true},
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
			FileTemplate: "",
		},
	}

	for _, tc := range fileFormatTestCases {
		secretsGroupTestCase.contents["conjur.org/secret-file-format.cache"] = tc.input
		group := resultSecretsGroups["cache"]
		group.FileFormat = tc.output

		if tc.hasTemplate {
			secretsGroupTestCase.contents["conjur.org/secret-file-template.cache"] = someValidTemplate
			group.FileTemplate = someValidTemplate
		} else {
			secretsGroupTestCase.contents["conjur.org/secret-file-template.cache"] = ""
			group.FileTemplate = ""
		}

		if tc.valid {
			secretsGroupTestCase.assert = assertExpectedSecretsGroups(resultSecretsGroups)
		} else {
			secretsGroupTestCase.assert = assertProperError("unknown file format")
		}

		resultSecretsGroups["cache"] = group

		t.Run(secretsGroupTestCase.description, func(t *testing.T) {
			secretGroups, err := ExtractSecretsGroupsFromAnnotations(secretsGroupTestCase.contents)
			secretsGroupTestCase.assert(t, secretGroups, err)
		})
	}
}
