package pushtofile

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pushToWriterTestCase struct {
	description string
	template    string
	secrets     []*Secret
	assert      func (*testing.T, string, error)
}

func (tc pushToWriterTestCase) Run(t *testing.T) {
	t.Run(tc.description, func(t *testing.T) {
		buf := new(bytes.Buffer)
		err := pushToWriter(
			buf,
			"group path",
			tc.template,
			tc.secrets,
		)
		tc.assert(t, buf.String(), err)
	})
}

func assertGoodOutput(expected string) func (*testing.T, string, error) {
	return func(t *testing.T, actual string, err error) {
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(
			t,
			expected,
			actual,
		)
	}
}

var writeToFileTestCases = []pushToWriterTestCase{
	{
		description: "happy path",
		template: `{{secret "alias"}}`,
		secrets: []*Secret{{Alias: "alias", Value: "secret value"}},
		assert:  assertGoodOutput("secret value"),
	},
	{
		description: "undefined secret",
		template: `{{secret "x"}}`,
		secrets: []*Secret{{Alias: "alias", Value: "secret value"}},
		assert: func(t *testing.T, s string, err error) {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "secret alias not present in specified secrets for group")
		},
	},
}

func Test_pushToWriter(t *testing.T) {
	for _, tc := range writeToFileTestCases {
		tc.Run(t)
	}
}

