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
	invalidBashVarName = "Must be alphanumerics and underscores"
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

func assertExpErrStr(t *testing.T, desc string, err error, expErrStr string) {
	if expErrStr == "" {
		assert.NoError(t, err, desc)
	} else {
		assert.Error(t, err, desc)
		assert.Contains(t, err.Error(), expErrStr, desc)
	}
}

func generateLongName(len int, char byte) string {
	var b strings.Builder
	b.Grow(len)
	for i := 0; i < len; i++ {
		b.WriteByte(byte(char))
	}
	return b.String()
}

func TestValidateSecretSpecPath(t *testing.T) {
	repeatChar := []byte("a")[0]
	maxLenConjurVarName := generateLongName(maxConjurVarNameLen, repeatChar)

	testCases := []struct {
		description    string
		path           string
		expectedErrStr string
	}{
		{
			"valid Conjur path",
			validConjurPath,
			"",
		}, {
			"null Conjur path",
			"",
			"null Conjur variable path",
		}, {
			"trailing '/' in Conjur path",
			validConjurPath + "/",
			"has a trailing '/'",
		}, {
			"Conjur path with max len var name",
			validConjurPath + "/" + maxLenConjurVarName,
			"",
		}, {
			"Conjur path with oversized var name",
			validConjurPath + "/" + maxLenConjurVarName + "a",
			fmt.Sprintf("is longer than %d characters", maxConjurVarNameLen),
		},
	}

	for _, tc := range testCases {
		// Set up test case
		secretSpec := SecretSpec{Alias: "foobar", Path: tc.path}

		// Run test case
		err := validateSecretSpec(secretSpec, "yaml")

		// Check result
		assertExpErrStr(t, tc.description, err, tc.expectedErrStr)
	}
}

func TestValidateSecretSpecAliasChars(t *testing.T) {
	testCases := []struct {
		fileFormat     string
		description    string
		testChar       rune
		expectedErrStr string
	}{
		// YAML file format, 8-bit characters
		{"yaml", "printable ASCII", '\u003F', ""},
		{"yaml", "heart emoji", '💙', ""},
		{"yaml", "dog emoji", '🐶', ""},
		{"yaml", "ASCII NULL", '\u0000', invalidYAMLChar},
		{"yaml", "ASCII BS", '\u0008', invalidYAMLChar},
		{"yaml", "ASCII tab", '\u0009', ""},
		{"yaml", "ASCII LF", '\u000A', ""},
		{"yaml", "ASCII VT", '\u000B', invalidYAMLChar},
		{"yaml", "ASCII CR", '\u000D', ""},
		{"yaml", "ASCII space", '\u0020', ""},
		{"yaml", "ASCII tilde", '\u007E', ""},
		{"yaml", "ASCII DEL", '\u007F', invalidYAMLChar},
		// YAML file format, 16-bit Unicode
		{"yaml", "Unicode NEL", '\u0085', ""},
		{"yaml", "Unicode 0x86", '\u0086', invalidYAMLChar},
		{"yaml", "Unicode 0x9F", '\u009F', invalidYAMLChar},
		{"yaml", "Unicode 0xA0", '\u00A0', ""},
		{"yaml", "Unicode 0xD7FF", '\uD7FF', ""},
		{"yaml", "Unicode 0xE000", '\uE000', ""},
		{"yaml", "Unicode 0xFFFD", '\uFFFD', ""},
		{"yaml", "Unicode 0xFFFE", '\uFFFE', invalidYAMLChar},
		// YAML file format, 32-bit Unicode
		{"yaml", "Unicode 0x10000", '\U00010000', ""},
		{"yaml", "Unicode 0x10FFFF", '\U0010FFFF', ""},

		// JSON file format, valid characters
		{"json", "ASCII NUL", '\u0000', invalidJSONChar},
		{"json", "ASCII 0x1F", '\u001F', invalidJSONChar},
		{"json", "ASCII space", '\u0020', ""},
		{"json", "ASCII tilde", '~', ""},
		{"json", "heart emoji", '💙', ""},
		{"json", "dog emoji", '🐶', ""},
		{"json", "Unicode 0x10000", '\U00010000', ""},
		{"json", "Unicode 0x10FFFF", '\U0010FFFF', ""},
		// JSON file format, invalid characters
		{"json", "ASCII NULL", '\u0000', invalidJSONChar},
		{"json", "ASCII BS", '\u0008', invalidJSONChar},
		{"json", "ASCII tab", '\u0009', invalidJSONChar},
		{"json", "ASCII LF", '\u000A', invalidJSONChar},
		{"json", "ASCII VT", '\u000B', invalidJSONChar},
		{"json", "ASCII DEL", '\u007F', invalidJSONChar},
		{"json", "ASCII quote", '"', invalidJSONChar},
		{"json", "ASCII backslash", '\\', invalidJSONChar},
	}

	for _, tc := range testCases {
		// Set up test case
		desc := fmt.Sprintf("%s file format, key containing %s character",
			tc.fileFormat, tc.description)
		alias := "key_containing_" + string(tc.testChar) + "_character"
		secretSpec := SecretSpec{Alias: alias, Path: validConjurPath}

		// Run test case
		err := validateSecretSpec(secretSpec, tc.fileFormat)

		// Check result
		assertExpErrStr(t, desc, err, tc.expectedErrStr)
	}
}

func TestValidateSecretSpecBashAliases(t *testing.T) {
	testFormats := []string{"bash", "dotenv"}
	testCases := []struct {
		description    string
		alias          string
		expectedErrStr string
	}{
		// Bash file format, valid aliases
		{"all lower case chars", "foobar", ""},
		{"all upper case chars", "FOOBAR", ""},
		{"upper case, lower case, and underscores", "_Foo_Bar_", ""},
		{"leading underscore with digits", "_12345", ""},
		{"upper case, lower case, underscores, digits", "_Foo_Bar_1234", ""},

		// Bash file format, invalid aliases
		{"leading digit", "7th_Heaven", invalidBashVarName},
		{"spaces", "FOO BAR", invalidBashVarName},
		{"dashes", "FOO-BAR", invalidBashVarName},
		{"single quotes", "FOO_'BAR'", invalidBashVarName},
		{"dog emoji", "FOO_'🐶'_BAR", invalidBashVarName},
		{"trailing space", "FOO_BAR ", invalidBashVarName},
	}

	for _, fileFormat := range testFormats {
		for _, tc := range testCases {
			// Set up test case
			desc := fmt.Sprintf("%s file format, key with %s",
				fileFormat, tc.description)
			secretSpec := SecretSpec{Alias: tc.alias, Path: validConjurPath}

			// Run test case
			err := validateSecretSpec(secretSpec, fileFormat)

			// Check result
			assertExpErrStr(t, desc, err, tc.expectedErrStr)
		}
	}
}

func TestValidateSecretSpecAliasLen(t *testing.T) {
	repeatChar := []byte("a")[0]
	maxLenYAMLAlias := generateLongName(maxYAMLKeyLen, repeatChar)
	maxLenJSONAlias := generateLongName(maxJSONKeyLen, repeatChar)

	testCases := []struct {
		fileFormat     string
		desc           string
		alias          string
		expectedErrStr string
	}{
		// YAML file format
		{"yaml", "single char alias", "a", ""},
		{"yaml", "maximum length  alias", maxLenYAMLAlias, ""},
		{"yaml", "oversized alias", maxLenYAMLAlias + "a", yamlAliasTooLong},

		// JSON file format
		{"json", "single-char alias", "a", ""},
		{"json", "maximum length alias", maxLenJSONAlias, ""},
		{"json", "oversized alias", maxLenJSONAlias + "a", jsonAliasTooLong},
	}

	for _, tc := range testCases {
		// Set up test case
		desc := fmt.Sprintf("%s file format, %s", tc.fileFormat, tc.desc)
		secretSpec := SecretSpec{Alias: tc.alias, Path: validConjurPath}

		// Run test case
		err := validateSecretSpec(secretSpec, tc.fileFormat)

		// Check result
		assertExpErrStr(t, desc, err, tc.expectedErrStr)
	}
}
