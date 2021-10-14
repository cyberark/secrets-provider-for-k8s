package push_to_file

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pushToFileWithDepsTestCase struct {
	description            string
	group                  SecretGroup
	overrideSecrets        []*Secret // Overrides secrets generated from group secret specs
	toWriterPusherErr      error
	toWriteCloserOpenerErr error
	assert                 func(t *testing.T, spyOpen toWriteCloserOpenerSpy, closableBuf *ClosableBuffer, spyPush toWriterPusherSpy, err error)
}

func (tc *pushToFileWithDepsTestCase) Run(t *testing.T) {
	t.Run(tc.description, func(t *testing.T) {
		// Input
		group := tc.group

		// Setup mocks
		closableBuf := new(ClosableBuffer)
		spyPushToWriter := toWriterPusherSpy{
			err: tc.toWriterPusherErr,
		}
		spyOpenWriteCloser := toWriteCloserOpenerSpy{
			writeCloser: closableBuf,
			err:         tc.toWriteCloserOpenerErr,
		}

		// Use secrets from group or override
		var secrets []*Secret
		if tc.overrideSecrets != nil {
			secrets = tc.overrideSecrets
		} else {
			secrets = make([]*Secret, len(group.SecretSpecs))
			for i, spec := range group.SecretSpecs {
				secrets[i] = &Secret{
					Alias: spec.Alias,
					Value: "value-" + spec.Path,
				}
			}
		}

		// Exercise
		err := group.pushToFileWithDeps(
			spyPushToWriter.Call,
			spyOpenWriteCloser.Call,
			secrets)

		tc.assert(t, spyOpenWriteCloser, closableBuf, spyPushToWriter, err)
	})
}

func modifyGoodGroup(modifiers ...func(SecretGroup) SecretGroup) SecretGroup {
	group := SecretGroup{
		Name:            "groupname",
		FilePath:        "path/to/file",
		FileTemplate:    "filetemplate",
		FileFormat:      "yaml",
		FilePermissions: 123,
		SecretSpecs:     goodSecretSpecs(),
	}

	for _, modifier := range modifiers {
		group = modifier(group)
	}

	return group
}

func goodSecretSpecs() []SecretSpec {
	return []SecretSpec{
		{
			Alias: "alias1",
			Path: "path1",
		},
		{
			Alias: "alias2",
			Path: "path2",
		},
	}
}

func TestNewSecretGroups(t *testing.T) {
	t.Run("valid annotations", func(t *testing.T) {
		secretGroups, err := NewSecretGroups(map[string]string{
			"conjur.org/conjur-secrets.first": `- path/to/secret/first1
- aliasfirst2: path/to/secret/first2`,
			"conjur.org/secret-file-path.first":      "firstfilepath",
			"conjur.org/secret-file-template.first":  `firstfiletemplate`,
			"conjur.org/conjur-secrets.second":       "- path/to/secret/second",
			"conjur.org/secret-file-path.second":     "secondfilepath",
			"conjur.org/secret-file-template.second": `secondfiletemplate`,
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Len(t, secretGroups, 2)
		assert.Equal(t, *secretGroups[0], SecretGroup{
			Name:            "first",
			FilePath:        "firstfilepath",
			FileTemplate:    "firstfiletemplate",
			FileFormat:      "",
			FilePermissions: defaultFilePermissions,
			SecretSpecs: []SecretSpec{
				{
					Alias: "first1",
					Path:  "path/to/secret/first1",
				},
				{
					Alias: "aliasfirst2",
					Path:  "path/to/secret/first2",
				},
			},
		})
		assert.Equal(t, *secretGroups[1], SecretGroup{
			Name:            "second",
			FilePath:        "secondfilepath",
			FileTemplate:    "secondfiletemplate",
			FileFormat:      "",
			FilePermissions: defaultFilePermissions,
			SecretSpecs: []SecretSpec{
				{
					Alias: "second",
					Path:  "path/to/secret/second",
				},
			},
		})

	})

	t.Run("invalid secret specs annotation", func(t *testing.T) {
		_, err := NewSecretGroups(map[string]string{
			"conjur.org/conjur-secrets.first": `gibberish`,
			"conjur.org/secret-file-path.first":      "firstfilepath",
			"conjur.org/secret-file-template.first":  `firstfiletemplate`,
			"conjur.org/conjur-secrets.second":       "- path/to/secret/second",
			"conjur.org/secret-file-path.second":     "secondfilepath",
			"conjur.org/secret-file-template.second": `secondfiletemplate`,
		})

		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), `unable to create secret specs from annotation "conjur.org/conjur-secrets.first"`)
		assert.Contains(t, err.Error(), "cannot unmarshall to list of secret specs")
	})
}

var pushToFileWithDepsTestCases = []pushToFileWithDepsTestCase{
	{
		description:     "happy path",
		group:           modifyGoodGroup(),
		overrideSecrets: nil,
		assert: func(
			t *testing.T,
			spyOpen toWriteCloserOpenerSpy,
			closableBuf *ClosableBuffer,
			spyPush toWriterPusherSpy,
			err error,
		) {
			// Assertions
			assert.NoError(t, err)
			// Assert on toWriterPusher
			assert.Equal(t, "groupname", spyPush.args.groupName, )
			assert.Equal(t, closableBuf, spyPush.args.writer)
			assert.Equal(t, "filetemplate", spyPush.args.groupTemplate)
			assert.Equal(t, spyPush.args.groupSecrets, []*Secret{
				{
					Alias: "alias1",
					Value: "value-path1",
				},
				{
					Alias: "alias2",
					Value: "value-path2",
				},
			})
			// Assert on WriteCloserOpener
			assert.Equal(t, spyOpen.args.path, "path/to/file")
			assert.EqualValues(t, spyOpen.args.permissions, 123)
		},
	},
	{
		description: "missing file format or template",
		group: modifyGoodGroup(func(group SecretGroup) SecretGroup {
			group.FileTemplate = ""
			group.FileFormat = ""

			return group
		}),
		overrideSecrets: nil,
		assert: func(
			t *testing.T,
			spyOpen toWriteCloserOpenerSpy,
			closableBuf *ClosableBuffer,
			spyPush toWriterPusherSpy,
			err error,
		) {
			// Assertions
			assert.Error(t, err)
			assert.Equal(t, err.Error(), `missing one of "file template" or "file format" for group`)
		},
	},
	{
		description:     "secrets list is empty",
		group:           modifyGoodGroup(),
		overrideSecrets: []*Secret{},
		assert: func(
			t *testing.T,
			spyOpen toWriteCloserOpenerSpy,
			closableBuf *ClosableBuffer,
			spyPush toWriterPusherSpy,
			err error,
		) {
			// Assertions
			if !assert.Error(t, err) {
				return
			}
			assert.Contains(t, err.Error(), `number of secrets (0) does not match number of secret specs (2)`)
		},
	},
	{
		description: "file template precedence",
		group: modifyGoodGroup(func(group SecretGroup) SecretGroup {
			group.FileTemplate = "setfiletemplate"
			group.FileFormat = "setfileformat"

			return group
		}),
		overrideSecrets: nil,
		assert: func(
			t *testing.T,
			spyOpen toWriteCloserOpenerSpy,
			closableBuf *ClosableBuffer,
			spyPush toWriterPusherSpy,
			err error,
		) {
			// Assertions
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, spyPush.args.groupTemplate, `setfiletemplate`)
		},
	},
}

func TestSecretGroup_pushToFileWithDeps(t *testing.T) {
	for _, tc := range pushToFileWithDepsTestCases {
		tc.Run(t)
	}

	for _, format := range []string{"json", "yaml", "bash", "dotenv"} {
		tc := pushToFileWithDepsTestCase{
			description: fmt.Sprintf("%s format", format),
			group: modifyGoodGroup(func(group SecretGroup) SecretGroup {
				group.FileTemplate = ""
				group.FileFormat = format
				fmt.Println(group.SecretSpecs)
				return group
			}),
			overrideSecrets: nil,
			assert: func(
				t *testing.T,
				spyOpen toWriteCloserOpenerSpy,
				closableBuf *ClosableBuffer,
				spyPush toWriterPusherSpy,
				err error,
			) {
				// Assertions
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, spyPush.args.groupTemplate, standardTemplates[format].Template)
			},
		}

		tc.Run(t)
	}
}

func TestSecretGroup_PushToFile(t *testing.T) {
	// Create temp directory
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer os.Remove(dir)
	filePath := dir + "file."

	// Create a group, and push to file
	group := SecretGroup{
		Name:            "groupname",
		FilePath:        filePath,
		FileTemplate:    "",
		FileFormat:      "yaml",
		FilePermissions: 0744,
		SecretSpecs: []SecretSpec{
			{
				Alias: "alias1",
				Path:  "path1",
			},
			{
				Alias: "alias2",
				Path:  "path2",
			},
		},
	}
	err = group.PushToFile([]*Secret{
		{
			Alias: "alias1",
			Value: "value1",
		},
		{
			Alias: "alias2",
			Value: "value2",
		},
	})
	assert.NoError(t, err)

	// Read file contents and metadata
	contentBytes, err := ioutil.ReadFile(filePath)
	assert.NoError(t, err)
	f, err := os.Stat(filePath)
	assert.NoError(t, err)

	// Assert on file contents and metadata
	assert.EqualValues(t, f.Mode(), 0744)
	assert.Equal(t,
		`"alias1": "value1"
"alias2": "value2"`,
		string(contentBytes),
	)
}
