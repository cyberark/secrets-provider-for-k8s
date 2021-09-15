package annotations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type parseAnnotationsTestCase struct {
	description string
	contents    string
	assert      func(t *testing.T, result map[string]string, err error)
}

func assertGoodAnnotations(expected map[string]string) func(*testing.T, map[string]string, error) {
	return func(t *testing.T, result map[string]string, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, expected, result)
	}
}

func assertEmptyMap() func(*testing.T, map[string]string, error) {
	return func(t *testing.T, result map[string]string, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, map[string]string{}, result)
	}
}

func assertProperError(expectedErr string) func(*testing.T, map[string]string, error) {
	return func(t *testing.T, result map[string]string, err error) {
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), expectedErr)
	}
}

var parseAnnotationsTestCases = []parseAnnotationsTestCase{
	{
		description: "valid file",
		contents: `conjur.org/authn-identity="host/conjur/authn-k8s/cluster/apps/inventory-api"
conjur.org/container-mode="init"
conjur.org/secrets-destination="k8s_secret"
conjur.org/k8s-secrets="- k8s-secret-1\n- k8s-secret-2\n"
conjur.org/retry-count-limit="10"
conjur.org/retry-interval-sec="5"
conjur.org/debug-logging="true"
conjur.org/conjur-secrets.this-group="- test/url\n- test-password: test/password\n- test-username: test/username\n"
conjur.org/secret-file-path.this-group="this-relative-path"
conjur.org/secret-file-format.this-group="yaml"`,
		assert: assertGoodAnnotations(
			map[string]string{
				"conjur.org/authn-identity":                "host/conjur/authn-k8s/cluster/apps/inventory-api",
				"conjur.org/container-mode":                "init",
				"conjur.org/secrets-destination":           "k8s_secret",
				"conjur.org/k8s-secrets":                   "- k8s-secret-1\n- k8s-secret-2\n",
				"conjur.org/retry-count-limit":             "10",
				"conjur.org/retry-interval-sec":            "5",
				"conjur.org/debug-logging":                 "true",
				"conjur.org/conjur-secrets.this-group":     "- test/url\n- test-password: test/password\n- test-username: test/username\n",
				"conjur.org/secret-file-path.this-group":   "this-relative-path",
				"conjur.org/secret-file-format.this-group": "yaml",
			},
		),
	},
	{
		description: "test other explicitely allowed annotation values",
		contents: `conjur.org/container-mode="application"
conjur.org/secrets-destination="file"`,
		assert: assertGoodAnnotations(
			map[string]string{
				"conjur.org/container-mode":      "application",
				"conjur.org/secrets-destination": "file",
			},
		),
	},
	{
		description: "malformed annotation file line with unquoted value",
		contents:    "conjur.org/container-mode=application",
		assert:      assertProperError("Annotation file line 1 is malformed"),
	},
	{
		description: "malformed annotation file line without '='",
		contents: `conjur.org/container-mode="application"
conjur.org/retry-count-limit: 5`,
		assert: assertProperError("Annotation file line 2 is malformed"),
	},
	{
		description: "unrecognized annotation keys",
		contents: `conjur.org/valid-but-unrecognized="good-value"
invalid.org/container-mode="init"
sample-bad-key="good-value"`,
		assert: assertEmptyMap(),
	},
	{
		description: "invalid annotation value type for Integer annotation",
		contents:    "conjur.org/retry-count-limit=\"seven\"",
		assert:      assertProperError("must be type Integer"),
	},
	{
		description: "invalid annotation value type for Boolean annotation",
		contents:    "conjur.org/debug-logging=\"not-a-boolean\"",
		assert:      assertProperError("must be type Boolean"),
	},
	{
		description: "invalid annotation value for 'conjur.org/container-mode'",
		contents:    "conjur.org/container-mode=\"bad-container-mode\"",
		assert:      assertProperError("only accepts [init application]"),
	},
	{
		description: "invald annotation value for 'conjur.org/secrets-destination'",
		contents:    "conjur.org/secrets-destination=\"bad-destination\"",
		assert:      assertProperError("only accepts [file k8s_secret]"),
	},
	{
		description: "invalid annotation value for 'conjur.org/secret-file-format.{secrets_group}",
		contents:    "conjur.org/secret-file-format.this-group=\"bad-format\"",
		assert:      assertProperError("only accepts [yaml json dotenv bash]"),
	},
}

func TestParseAnnotation(t *testing.T) {
	for _, tc := range parseAnnotationsTestCases {
		t.Run(tc.description, func(t *testing.T) {
			annotations, err := ParseAnnotations(strings.NewReader(tc.contents))
			tc.assert(t, annotations, err)
		})
	}
}
