package pushtofile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: Some thoughts on the unit tests
// + Separate test cases by file format (vs length or character validity)
// + Avoid holding expectations as fields on test case definition. Having a closure that does the
// actual assertion aids in debugging by having the test trace point you to the test case
//

func assertExpErrStr(t *testing.T, desc string, err error, expErrStr string) {
	if expErrStr == "" {
		assert.NoError(t, err, desc)
	} else {
		if !assert.Error(t, err, desc) {
			return
		}
		assert.Contains(t, err.Error(), expErrStr, desc)
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

		// Run test case
		err := standardTemplates[tc.fileFormat].validateAlias(alias)

		// Check result
		assertExpErrStr(t, desc, err, tc.expectedErrStr)
	}
}

func TestValidateSecretSpecAliasLen(t *testing.T) {
	maxLenYAMLAlias := strings.Repeat("a", maxYAMLKeyLen)
	maxLenJSONAlias := strings.Repeat("a", maxJSONKeyLen)

	testCases := []struct {
		fileFormat     string
		desc           string
		alias          string
		expectedErrStr string // empty string means no error
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

		// Run test case
		err := standardTemplates[tc.fileFormat].validateAlias(tc.alias)

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
			desc := fmt.Sprintf("%q file format, key with %q",
				fileFormat, tc.description)

			// Run test case
			err := standardTemplates[fileFormat].validateAlias(tc.alias)

			// Check result
			assertExpErrStr(t, desc, err, tc.expectedErrStr)
		}
	}
}