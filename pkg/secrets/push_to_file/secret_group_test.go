package push_to_file

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSecretGroups(t *testing.T) {
	t.Run("normal test", func(t *testing.T) {
		secretGroups, errs := NewSecretGroups(map[string]string{
			"conjur.org/conjur-secrets.first": `- path/to/secret/first1
- aliasfirst2: path/to/secret/first2`,
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
}

type ClosableBuffer struct {
	bytes.Buffer
	CloseErr error
}

func (c ClosableBuffer) Close() error { return c.CloseErr }

type toWriterPusherArgs struct {
	writer        io.Writer
	groupName     string
	groupTemplate string
	groupSecrets  []*Secret
}

type toWriterPusherSpy struct {
	args   toWriterPusherArgs
	err    error
	_calls int
}

func (spy *toWriterPusherSpy) Call(
	writer io.Writer,
	groupName string,
	groupTemplate string,
	groupSecrets []*Secret,
) error {
	spy._calls++
	if spy._calls > 1 {
		panic("spy called more than once")
	}

	spy.args = toWriterPusherArgs{
		writer:        writer,
		groupName:     groupName,
		groupTemplate: groupTemplate,
		groupSecrets:  groupSecrets,
	}

	return spy.err
}

type toWriteCloserOpenerArgs struct {
	path        string
	permissions os.FileMode
}

type toWriteCloserOpenerSpy struct {
	args        toWriteCloserOpenerArgs
	writeCloser io.WriteCloser
	err         error
	_calls      int
}

func (spy *toWriteCloserOpenerSpy) Call(path string, permissions os.FileMode) (io.WriteCloser, error) {
	spy._calls++
	if spy._calls > 1 {
		panic("spy called more than once")
	}

	spy.args = toWriteCloserOpenerArgs{
		path:        path,
		permissions: permissions,
	}

	return spy.writeCloser, spy.err
}

type pushToFileWithDepsTestCase struct {
	description string
	group       SecretGroup
	secrets     []*Secret
	assert      func(t *testing.T, spyOpen toWriteCloserOpenerSpy, closableBuf *ClosableBuffer, spyPush toWriterPusherSpy, err error)
}

func (tc *pushToFileWithDepsTestCase) Run(t *testing.T) {
	t.Run(tc.description, func(t *testing.T) {
		// Input
		group := tc.group

		// Setup mocks
		closableBuf := new(ClosableBuffer)
		spyPushToWriter := toWriterPusherSpy{}
		spyOpenWriteCloser := toWriteCloserOpenerSpy{
			writeCloser: closableBuf,
		}

		// Exercise
		err := group.pushToFileWithDeps(
			spyPushToWriter.Call,
			spyOpenWriteCloser.Call,
			tc.secrets)

		tc.assert(t, spyOpenWriteCloser, closableBuf, spyPushToWriter, err)
	})
}

var pushToFileWithDepsTestCases = []pushToFileWithDepsTestCase{
	{
		description: "happy path",
		group: SecretGroup{
			Name:            "group path",
			FilePath:        "path/to/file",
			FileTemplate:    "{ xyz }",
			FileFormat:      "json",
			FilePermissions: 123,
		},
		secrets: []*Secret{
			{
				Alias: "alias1",
				Value: "value1",
			},
		},
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
			assert.Equal(t, spyPush.args.groupName, "group path")
			assert.Equal(t, spyPush.args.writer, closableBuf)
			assert.Equal(t, spyPush.args.groupTemplate, `{ xyz }`)
			assert.Equal(t, spyPush.args.groupSecrets, []*Secret{
				{
					Alias: "alias1",
					Value: "value1",
				},
			})
			// Assert on WriteCloserOpener
			assert.Equal(t, spyOpen.args.path, "path/to/file")
			assert.EqualValues(t, spyOpen.args.permissions, 123)
		},
	},
	{
		description: "missing format or template",
		group: SecretGroup{
			Name:            "group path",
			FilePath:        "path/to/file",
			FileTemplate:    "",
			FileFormat:      "",
			FilePermissions: 123,
		},
		secrets: []*Secret{
			{
				Alias: "alias1",
				Value: "value1",
			},
		},
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
		description: "0 secrets",
		group: SecretGroup{
			Name:            "group path",
			FilePath:        "path/to/file",
			FileTemplate:    "x",
			FileFormat:      "",
			FilePermissions: 123,
			SecretSpecs:     nil,
		},
		secrets: nil,
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
			assert.Contains(t, err.Error(), `empty`)
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
			group: SecretGroup{
				Name:            "group path",
				FilePath:        "path/to/file",
				FileTemplate:    "",
				FileFormat:      format,
				FilePermissions: 123,
			},
			secrets: []*Secret{{}},
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
