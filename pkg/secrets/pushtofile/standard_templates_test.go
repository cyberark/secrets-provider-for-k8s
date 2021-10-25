package pushtofile

import (
	"testing"
)

var standardTemplateTestCases = []pushToWriterTestCase{
	{
		description: "json",
		template: standardTemplates["json"].template,
		secrets: []*Secret{
			{Alias: "alias 1", Value: "secret value 1"},
			{"alias 2", "secret value 2"},
		},
		assert:  assertGoodOutput(`{"alias 1":"secret value 1","alias 2":"secret value 2"}`),
	},
	{
		description: "yaml",
		template: standardTemplates["yaml"].template,
		secrets: []*Secret{
			{Alias: "alias 1", Value: "secret value 1"},
			{"alias 2", "secret value 2"},
		},
		assert:  assertGoodOutput(`"alias 1": "secret value 1"
"alias 2": "secret value 2"`),
	},
	{
		description: "dotenv",
		template: standardTemplates["dotenv"].template,
		secrets: []*Secret{
			{Alias: "alias1", Value: "secret value 1"},
			{"alias2", "secret value 2"},
		},
		assert:  assertGoodOutput(`alias1="secret value 1"
alias2="secret value 2"`),
	},
	{
		description: "bash",
		template: standardTemplates["bash"].template,
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
		tc.Run(t)
	}
}
