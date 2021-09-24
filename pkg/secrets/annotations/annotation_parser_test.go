package annotations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type assertFunc func(t *testing.T, result map[string]string, err error)

type parseReaderTestCase struct {
	description string
	contents    string
	assert      assertFunc
}

func assertGoodAnnotations(expected map[string]string) assertFunc {
	return func(t *testing.T, result map[string]string, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, expected, result)
	}
}

func assertEmptyMap() assertFunc {
	return func(t *testing.T, result map[string]string, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, map[string]string{}, result)
	}
}

func assertProperError(expectedErr string) assertFunc {
	return func(t *testing.T, result map[string]string, err error) {
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), expectedErr)
	}
}

var parseReaderTestCases = []parseReaderTestCase{
	{
		description: "valid file",
		contents: `conjur.org/authn-identity="host/conjur/authn-k8s/cluster/apps/inventory-api"
conjur.org/container-mode="init"
conjur.org/secrets-destination="k8s_secrets"
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
				"conjur.org/secrets-destination":           "k8s_secrets",
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
		description: "an empty annotations file results in an empty map",
		contents:    "",
		assert:      assertEmptyMap(),
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
}

func TestParseReader(t *testing.T) {
	for _, tc := range parseReaderTestCases {
		t.Run(tc.description, func(t *testing.T) {
			annotations, err := ParseReader(strings.NewReader(tc.contents))
			tc.assert(t, annotations, err)
		})
	}
}
