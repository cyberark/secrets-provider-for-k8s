package push_to_file

import (
	"bytes"
	"testing"
)

var standardTemplateTestCases = []pushToWriterTestCase{
	{
		description: "json",
		template: standardTemplates["json"].Template,
		secrets: []*Secret{
			{Alias: "alias 1", Value: "secret value 1"},
			{"alias 2", "secret value 2"},
		},
		assert:  assertGoodOutput(`{"alias 1":"secret value 1","alias 2":"secret value 2"}`),
	},
	{
		description: "yaml",
		template: standardTemplates["yaml"].Template,
		secrets: []*Secret{
			{Alias: "alias 1", Value: "secret value 1"},
			{"alias 2", "secret value 2"},
		},
		assert:  assertGoodOutput(`"alias 1": "secret value 1"
"alias 2": "secret value 2"`),
	},
	{
		description: "dotenv",
		template: standardTemplates["dotenv"].Template,
		secrets: []*Secret{
			{Alias: "alias1", Value: "secret value 1"},
			{"alias2", "secret value 2"},
		},
		assert:  assertGoodOutput(`alias1="secret value 1"
alias2="secret value 2"`),
	},
	{
		description: "bash",
		template: standardTemplates["bash"].Template,
		secrets: []*Secret{
			{Alias: "alias1", Value: "secret value 1"},
			{"alias2", "secret value 2"},
		},
		assert:  assertGoodOutput(`export alias1="secret value 1"
export alias2="secret value 2"`),
	},
}

func Test_standardTemplates(t *testing.T) {
	for _, tc := range standardTemplateTestCases {
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
}
