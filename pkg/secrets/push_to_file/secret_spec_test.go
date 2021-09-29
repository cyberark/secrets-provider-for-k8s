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
		description: "valid example",
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
		description: "malformed not a list",
		contents: `
admin-password: dev/openshift/password
another-password: dev/openshift/password
`,
		assert: func(t *testing.T, result []SecretSpec, err error) {
			assert.Contains(t, err.Error(), "failed to parse yaml list")
		},
	},
	{
		description: "malformed multiple key-values in one entry",
		contents: `
- dev/openshift/api-url
- admin-password: dev/openshift/password 
  another-admin-password: dev/openshift/password
`,
		assert: func(t *testing.T, result []SecretSpec, err error) {
			assert.Contains(t, err.Error(), "expected single key-value pair for secret specification")
		},
	},
}

func TestNewSecretSpecs(t *testing.T) {
	t.Run("normal test", func(t *testing.T) {
		res := 1 + 1
		assert.Equal(t, 2, res)
	})

	for _, tc := range secretsSpecTestCases {
		t.Run(tc.description, func(t *testing.T) {
			secretsSpecs, err := NewSecretSpecs([]byte(tc.contents))
			tc.assert(t, secretsSpecs, err)
		})
	}
}
