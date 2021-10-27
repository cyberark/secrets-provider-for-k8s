package pushtofile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	invalidYAMLChar    = "invalid YAML character"
	invalidJSONChar    = "invalid JSON character"
	yamlAliasTooLong   = "too long for YAML"
	jsonAliasTooLong   = "too long for JSON"
	invalidBashVarName = "must be alphanumerics and underscores"
	validConjurPath    = "valid/conjur/variable/path"
)

type secretsSpecTestCase struct {
	description string
	contents    string
	assert      func(t *testing.T, result []SecretSpec, err error)
}

func (tc secretsSpecTestCase) Run(t *testing.T) {
	t.Run(tc.description, func(t *testing.T) {
		secretsSpecs, err := NewSecretSpecs([]byte(tc.contents))
		tc.assert(t, secretsSpecs, err)
	})
}

func assertGoodSecretSpecs(expectedResult []SecretSpec) func(*testing.T, []SecretSpec, error) {
	return func(t *testing.T, result []SecretSpec, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(
			t,
			expectedResult,
			result,
		)
	}
}

var secretsSpecTestCases = []secretsSpecTestCase{
	{
		description: "valid secret spec formats",
		contents: `
- dev/openshift/api-url
- admin-password: dev/openshift/password
`,
		assert: assertGoodSecretSpecs(
			[]SecretSpec{
				{
					Alias: "api-url",
					Path:  "dev/openshift/api-url",
				},
				{
					Alias: "admin-password",
					Path:  "dev/openshift/password",
				},
			},
		),
	},
	{
		description: "secret specs are not a list",
		contents: `
admin-password: dev/openshift/password
another-password: dev/openshift/password
`,
		assert: func(t *testing.T, result []SecretSpec, err error) {
			assert.Contains(t, err.Error(), "cannot unmarshal")
			assert.Contains(t, err.Error(), "into []pushtofile.SecretSpec")
		},
	},
	{
		description: "secret spec map with multiple keys",
		contents: `
- admin-password: dev/openshift/password 
  another-admin-password: dev/openshift/password
- dev/openshift/api-url
`,
		assert: func(t *testing.T, result []SecretSpec, err error) {
			assert.Contains(t, err.Error(), "expected a")
			assert.Contains(t, err.Error(), "on line 2")
		},
	},
	{
		description: "secret spec map value is not a string",
		contents: `
- dev/openshift/api-url
- key: 
    inner-key: inner-value
`,
		assert: func(t *testing.T, result []SecretSpec, err error) {
			assert.Contains(t, err.Error(), "expected a")
			assert.Contains(t, err.Error(), "on line 3")
		},
	},
	{
		description: "unrecognized secret spec format",
		contents: `
- dev/openshift/api-url
- api-password: dev/openshift/api-password
- - list item
`,
		assert: func(t *testing.T, result []SecretSpec, err error) {
			assert.Contains(t, err.Error(), "expected a")
			assert.Contains(t, err.Error(), "on line 4")
		},
	},
}

func TestNewSecretSpecs(t *testing.T) {
	for _, tc := range secretsSpecTestCases {
		tc.Run(t)
	}
}

func Test_validateSecretPath(t *testing.T) {
	maxLenConjurVarName := strings.Repeat("a", maxConjurVarNameLen)

	testCases := []struct {
		description    string
		path           string
		expectedErrStr string
	}{
		{
			description:  "valid Conjur path",
			path:         validConjurPath,
		}, {
			description:    "empty Conjur path",
			path:           "",
			expectedErrStr: "must not be empty",
		}, {
			description:    `trailing "/" in Conjur path`,
			path:           validConjurPath + "/",
			expectedErrStr: `must not have a trailing "/"`,
		}, {
			description:  "Conjur path with max len var name",
			path:         validConjurPath + "/" + maxLenConjurVarName,
		}, {
			description:    "Conjur path with oversized var name",
			path:           validConjurPath + "/" + maxLenConjurVarName + "a",
			expectedErrStr: fmt.Sprintf("must not be longer than %d characters", maxConjurVarNameLen),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Set up and run test case
			err := validateSecretPath(tc.path)

			// Check result
			if len(tc.expectedErrStr) == 0 {
				assert.NoError(t, err)
				return
			}

			if !assert.Error(t, err) {
				return
			}
			assert.Contains(t, err.Error(), tc.expectedErrStr)
		})
	}
}
