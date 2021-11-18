package pushtofile

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pushToFileWithDepsTestCase struct {
	description            string
	group                  SecretGroup
	overrideSecrets        []*Secret // Overrides secrets generated from group secret specs
	overridePushToWriter   func(writer io.Writer, groupName string, groupTemplate string, groupSecrets []*Secret) error
	toWriterPusherErr      error
	toWriteCloserOpenerErr error
	assert                 func(t *testing.T, spyOpenWriteCloser openWriteCloserSpy, closableBuf *ClosableBuffer, spyPushToWriter pushToWriterSpy, err error)
}

func (tc *pushToFileWithDepsTestCase) Run(t *testing.T) {
	t.Run(tc.description, func(t *testing.T) {
		// Input
		group := tc.group

		// Setup mocks
		closableBuf := new(ClosableBuffer)
		spyPushToWriter := pushToWriterSpy{
			err: tc.toWriterPusherErr,
		}
		spyOpenWriteCloser := openWriteCloserSpy{
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

		pushToWriterFunc := spyPushToWriter.Call
		if tc.overridePushToWriter != nil {
			pushToWriterFunc = tc.overridePushToWriter
		}

		// Exercise
		err := group.pushToFileWithDeps(
			spyOpenWriteCloser.Call,
			pushToWriterFunc,
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
			Path:  "path1",
		},
		{
			Alias: "alias2",
			Path:  "path2",
		},
	}
}

func TestNewSecretGroups(t *testing.T) {
	t.Run("valid annotations", func(t *testing.T) {
		secretGroups, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets-policy-path.first": "/path/to/secret/",
			"conjur.org/conjur-secrets.first": `- first1
- aliasfirst2: first2`,
			"conjur.org/secret-file-path.first":      "firstfilepath",
			"conjur.org/secret-file-template.first":  `firstfiletemplate`,
			"conjur.org/conjur-secrets.second":       "- path/to/secret/second",
			"conjur.org/secret-file-path.second":     "secondfilepath",
			"conjur.org/secret-file-template.second": `secondfiletemplate`,
		})

		if !assert.Empty(t, errs) {
			return
		}
		assert.Len(t, secretGroups, 2)
		assert.Equal(t, *secretGroups[0], SecretGroup{
			Name:             "first",
			FilePath:         "/basepath/firstfilepath",
			FileTemplate:     "firstfiletemplate",
			FileFormat:       "",
			FilePermissions:  defaultFilePermissions,
			PolicyPathPrefix: "path/to/secret/",
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
			FilePath:        "/basepath/secondfilepath",
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
		_, errs := NewSecretGroups("", map[string]string{
			"conjur.org/conjur-secrets.first":        `gibberish`,
			"conjur.org/secret-file-path.first":      "firstfilepath",
			"conjur.org/secret-file-template.first":  `firstfiletemplate`,
			"conjur.org/conjur-secrets.second":       "- path/to/secret/second",
			"conjur.org/secret-file-path.second":     "secondfilepath",
			"conjur.org/secret-file-template.second": `secondfiletemplate`,
		})

		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), `unable to create secret specs from annotation "conjur.org/conjur-secrets.first"`)
		assert.Contains(t, errs[0].Error(), "cannot unmarshall to list of secret specs")
	})

	t.Run("absolute secret file path annotation", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `
- path/to/secret/first1
- aliasfirst2: path/to/secret/first2
`,
			"conjur.org/secret-file-path.first": "/absolute/path",
		})

		assert.Len(t, errs, 1)
		assert.Contains(
			t,
			errs[0].Error(),
			`requires relative path`,
		)
	})

	t.Run("file path longer than 255 characters", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `- path/to/secret/first1
- aliasfirst2: path/to/secret/first2`,
			"conjur.org/secret-file-path.first":      "firstfilepath",
			"conjur.org/secret-file-template.first":  `firstfiletemplate`,
			"conjur.org/conjur-secrets.second":       "- path/to/secret/second",
			"conjur.org/secret-file-path.second":     strings.Repeat("secondfile", 26),
			"conjur.org/secret-file-template.second": `secondfiletemplate`,
		})
		assert.Len(t, errs, 1)
		assert.Contains(
			t,
			errs[0].Error(),
			`filepath for secret group "second" must not be longer than 255 characters`,
		)
	})

	t.Run("duplicate file paths", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `- path/to/secret/first1
- aliasfirst2: path/to/secret/first2`,
			"conjur.org/secret-file-path.first":      "firstfilepath",
			"conjur.org/secret-file-template.first":  `firstfiletemplate`,
			"conjur.org/conjur-secrets.second":       "- path/to/secret/second",
			"conjur.org/secret-file-path.second":     "firstfilepath",
			"conjur.org/secret-file-template.second": `secondfiletemplate`,
			"conjur.org/conjur-secrets.third":        "- path/to/secret/third",
			"conjur.org/secret-file-path.third":      "firstfilepath",
			"conjur.org/secret-file-template.third":  `thirdfiletemplate`,
		})

		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), `duplicate filepath "/basepath/firstfilepath" for groups`)
		// The order of the groups in the error message is not deterministic, so don't check the order.
		assert.Contains(t, errs[0].Error(), "first")
		assert.Contains(t, errs[0].Error(), "second")
		assert.Contains(t, errs[0].Error(), "third")
	})

	t.Run("duplicate file path using default", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `- path/to/secret/first1
- aliasfirst2: path/to/secret/first2`,
			"conjur.org/secret-file-path.first":  "./relative/path/to/folder/",
			"conjur.org/conjur-secrets.second":   "- path/to/secret/second",
			"conjur.org/secret-file-path.second": "./relative/path/to/folder/first.yaml",
		})

		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), `duplicate filepath "/basepath/relative/path/to/folder/first.yaml" for groups`)
		// The order of the groups in the error message is not deterministic, so don't check the order.
		assert.Contains(t, errs[0].Error(), "first")
		assert.Contains(t, errs[0].Error(), "second")
	})

	t.Run("secret file template requires file path annotation as file", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `
- path/to/secret/first1
- aliasfirst2: path/to/secret/first2
`,
			"conjur.org/secret-file-path.first":     "./relative/path/to/folder/",
			"conjur.org/secret-file-template.first": "some template",
		})

		assert.Len(t, errs, 1)
		assert.Contains(
			t,
			errs[0].Error(),
			`path to a file`,
		)
	})

	t.Run("secret file template requires explicit file path", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `
- path/to/secret/first1
- aliasfirst2: path/to/secret/first2
`,
			"conjur.org/secret-file-template.first": "some template",
		})

		assert.Len(t, errs, 1)
		assert.Contains(
			t,
			errs[0].Error(),
			`path to a file`,
		)
	})

	t.Run("secret file path default", func(t *testing.T) {
		groups, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `
- path/to/secret/first1
- aliasfirst2: path/to/secret/first2
`,
		})

		assert.Len(t, errs, 0)
		assert.Len(t, groups, 1)
		assert.Equal(
			t,
			groups[0].FilePath,
			`/basepath/first.yaml`,
		)
	})

	t.Run("secret file path as directory default filename", func(t *testing.T) {
		groups, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `
- path/to/secret/first1
- aliasfirst2: path/to/secret/first2
`,
			"conjur.org/secret-file-path.first":   "./relative/path/to/folder/",
			"conjur.org/secret-file-format.first": "json",
		})

		assert.Len(t, errs, 0)
		assert.Len(t, groups, 1)
		assert.Equal(
			t,
			groups[0].FilePath,
			`/basepath/relative/path/to/folder/first.json`,
		)
	})

	t.Run("secret file path not relative to base", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `
- path/to/secret/first1
- aliasfirst2: path/to/secret/first2
`,
			"conjur.org/secret-file-path.first":   "../relative/path/to/parent/",
			"conjur.org/secret-file-format.first": "json",
		})

		assert.Len(t, errs, 1)
		assert.Contains(
			t,
			errs[0].Error(),
			"relative to secrets base path",
		)
	})

	t.Run("secret file format yaml default", func(t *testing.T) {
		groups, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `
- path/to/secret/first1
- aliasfirst2: path/to/secret/first2
`,
		})

		assert.Len(t, errs, 0)
		assert.Len(t, groups, 1)
		assert.Contains(
			t,
			groups[0].FileFormat,
			"yaml",
		)
	})

	t.Run("secret file path overrides default extension", func(t *testing.T) {
		groups, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first": `- path/to/secret/first1
- aliasfirst2: path/to/secret/first2`,
			"conjur.org/secret-file-path.first":   "./relative/path/to/folder/firstfilepath.json",
			"conjur.org/secret-file-format.first": "yaml",
		})

		assert.Len(t, errs, 0)
		assert.Len(t, groups, 1)
		assert.Equal(
			t,
			groups[0].FilePath,
			`/basepath/relative/path/to/folder/firstfilepath.json`,
		)
		assert.Contains(
			t,
			groups[0].FileFormat,
			"yaml",
		)

	})

	t.Run("fail custom format first-pass at parsing", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first":       "- path/to/secret/first1\n",
			"conjur.org/secret-file-path.first":     "firstfilepath",
			"conjur.org/secret-file-template.first": `{{ < }}`,
		})

		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), `unable to use file template for secret group "first"`)
		assert.NotContains(t, errs[0].Error(), `executing "first"`)
	})

	t.Run("fail custom format first-pass at execution", func(t *testing.T) {
		_, errs := NewSecretGroups("/basepath", map[string]string{
			"conjur.org/conjur-secrets.first":       "- path/to/secret/first1\n",
			"conjur.org/secret-file-path.first":     "firstfilepath",
			"conjur.org/secret-file-template.first": `{{ secret "x" }}`,
		})

		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), `unable to use file template for secret group "first"`)
		assert.Contains(t, errs[0].Error(), `executing "first"`)
		assert.Contains(t, errs[0].Error(), `secret alias "x" not present in specified secrets`)
	})
}

var pushToFileWithDepsTestCases = []pushToFileWithDepsTestCase{
	{
		description:          "happy path",
		group:                modifyGoodGroup(),
		overrideSecrets:      nil,
		overridePushToWriter: nil,
		assert: func(
			t *testing.T,
			spyOpenWriteCloser openWriteCloserSpy,
			closableBuf *ClosableBuffer,
			spyPushToWriter pushToWriterSpy,
			err error,
		) {
			// Assertions
			assert.NoError(t, err)
			// Assert on pushToWriterFunc
			assert.Equal(
				t,
				pushToWriterArgs{
					writer:        closableBuf,
					groupName:     "groupname",
					groupTemplate: "filetemplate",
					groupSecrets: []*Secret{
						{
							Alias: "alias1",
							Value: "value-path1",
						},
						{
							Alias: "alias2",
							Value: "value-path2",
						},
					},
				},
				spyPushToWriter.args,
			)
			// Assert on WriteCloserOpener
			assert.Equal(
				t,
				openWriteCloserArgs{
					path:        "path/to/file",
					permissions: 123,
				},
				spyOpenWriteCloser.args,
			)
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
			spyOpenWriteCloser openWriteCloserSpy,
			closableBuf *ClosableBuffer,
			spyPushToWriter pushToWriterSpy,
			err error,
		) {
			// Assertions
			if !assert.NoError(t, err) {
				return
			}
			// Defaults to yaml
			spyPushToWriter.args.groupTemplate = yamlTemplate
		},
	},
	{
		description:     "secrets list is empty",
		group:           modifyGoodGroup(),
		overrideSecrets: []*Secret{},
		assert: func(
			t *testing.T,
			spyOpenWriteCloser openWriteCloserSpy,
			closableBuf *ClosableBuffer,
			spyPushToWriter pushToWriterSpy,
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
			spyOpenWriteCloser openWriteCloserSpy,
			closableBuf *ClosableBuffer,
			spyPushToWriter pushToWriterSpy,
			err error,
		) {
			// Assertions
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, spyPushToWriter.args.groupTemplate, `setfiletemplate`)
		},
	},
	{
		description:     "template execution error",
		group:           modifyGoodGroup(),
		overrideSecrets: nil,
		overridePushToWriter: func(writer io.Writer, groupName string, groupTemplate string, groupSecrets []*Secret) error {
			return errors.New("underlying error message")
		},
		assert: func(
			t *testing.T,
			spyOpenWriteCloser openWriteCloserSpy,
			closableBuf *ClosableBuffer,
			spyPushToWriter pushToWriterSpy,
			err error,
		) {
			// Assertions
			if !assert.Error(t, err) {
				return
			}
			assert.Contains(t, err.Error(), `failed to execute template, with secret values, on push to file for secret group "groupname"`)
			assert.NotContains(t, err.Error(), "underlying error message")
		},
	},
	{
		description:     "template execution panic",
		group:           modifyGoodGroup(),
		overrideSecrets: nil,
		overridePushToWriter: func(writer io.Writer, groupName string, groupTemplate string, groupSecrets []*Secret) error {
			panic("canned panic response - maybe containing secrets")
		},
		assert: func(
			t *testing.T,
			spyOpenWriteCloser openWriteCloserSpy,
			closableBuf *ClosableBuffer,
			spyPushToWriter pushToWriterSpy,
			err error,
		) {
			// Assertions
			if !assert.Error(t, err) {
				return
			}
			assert.Contains(t, err.Error(), `failed to execute template, with secret values, on push to file for secret group "groupname"`)
			assert.NotContains(t, err.Error(), "canned panic response - maybe containing secrets")
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

				return group
			}),
			overrideSecrets: nil,
			assert: func(
				t *testing.T,
				spyOpenWriteCloser openWriteCloserSpy,
				closableBuf *ClosableBuffer,
				spyPushToWriter pushToWriterSpy,
				err error,
			) {
				// Assertions
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, spyPushToWriter.args.groupTemplate, standardTemplates[format].template)
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

	for _, tc := range []struct {
		description string
		path        string
	}{
		{"file with existing parent folder", "./file"},
		{"file with non-existent parent folder", "./path/to/file"},
	} {
		t.Run(tc.description, func(t *testing.T) {
			absoluteFilePath := path.Join(dir, tc.path)

			// Create a group, and push to file
			group := SecretGroup{
				Name:            "groupname",
				FilePath:        absoluteFilePath,
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
			contentBytes, err := ioutil.ReadFile(absoluteFilePath)
			assert.NoError(t, err)
			f, err := os.Stat(absoluteFilePath)
			assert.NoError(t, err)

			// Assert on file contents and metadata
			assert.EqualValues(t, f.Mode(), 0744)
			assert.Equal(t,
				`"alias1": "value1"
"alias2": "value2"`,
				string(contentBytes),
			)
		})
	}

	t.Run("failure to mkdir", func(t *testing.T) {
		// Create a group, and push to file
		group := SecretGroup{
			Name:            "groupname",
			FilePath:        "/dev/stdout/x",
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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to mkdir")
	})

	t.Run("failure to open file", func(t *testing.T) {
		// Create a group, and push to file
		group := SecretGroup{
			Name:            "groupname",
			FilePath:        "/dev/stdout",
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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to open file")
	})
}
