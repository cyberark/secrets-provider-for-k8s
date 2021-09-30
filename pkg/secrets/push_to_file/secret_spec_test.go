package push_to_file

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func assertGoodSecretSpecs(expectedResult []SecretSpec) func (*testing.T, []SecretSpec, error) {
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
					Path:  "dev/openshift/api-url",
					Alias: "api-url",
				},
				{
					Path:  "dev/openshift/password",
					Alias: "admin-password",
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
			assert.Contains(t, err.Error(), "into []push_to_file.SecretSpec")
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
