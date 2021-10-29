package pushtofile

import (
	"fmt"
	"testing"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/stretchr/testify/assert"
)

func retrieve(variableIDs []string) (map[string][]byte, error) {
	masterMap := make(map[string][]byte)
	for _, id := range variableIDs {
		masterMap[id] = []byte(fmt.Sprintf("value-%s", id))
	}
	return masterMap, nil
}

func TestNewProvider(t *testing.T) {
	TestCases := []struct {
		description         string
		retrieveFunc        conjur.RetrieveSecretsFunc
		basePath            string
		annotations         map[string]string
		expectedSecretGroup []*SecretGroup
	}{
		{
			description:  "happy case",
			retrieveFunc: retrieve,
			basePath:     "/basepath",
			annotations: map[string]string{
				"conjur.org/conjur-secrets.groupname":     "- password: path1\n",
				"conjur.org/secret-file-path.groupname":   "path/to/file",
				"conjur.org/secret-file-format.groupname": "yaml",
			},
			expectedSecretGroup: []*SecretGroup{
				{
					Name:            "groupname",
					FilePath:        "/basepath/path/to/file",
					FileTemplate:    "",
					FileFormat:      "yaml",
					FilePermissions: defaultFilePermissions,
					SecretSpecs: []SecretSpec{
						{
							Alias: "password",
							Path:  "path1",
						},
					},
				},
			},
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.description, func(t *testing.T) {
			p, err := NewProvider(tc.retrieveFunc, tc.basePath, tc.annotations)
			assert.Empty(t, err)
			assert.Equal(t, tc.expectedSecretGroup, p.secretGroups)
		})
	}
}

func TestProvideWithDeps(t *testing.T) {
	TestCases := []struct {
		description string
		provider    fileProvider
		assert      func(*testing.T, fileProvider, error, *ClosableBuffer, pushToWriterSpy, openWriteCloserSpy)
	}{
		{
			description: "happy path",
			provider: fileProvider{
				retrieveSecretsFunc: retrieve,
				secretGroups: []*SecretGroup{
					{
						Name:            "groupname",
						FilePath:        "/path/to/file",
						FileFormat:      "yaml",
						FilePermissions: 123,
						SecretSpecs: []SecretSpec{
							{
								Alias: "password",
								Path:  "path1",
							},
						},
					},
				},
			},
			assert: func(
				t *testing.T,
				p fileProvider,
				err error,
				closableBuf *ClosableBuffer,
				spyPushToWriter pushToWriterSpy,
				spyOpenWriteCloser openWriteCloserSpy,
			) {
				assert.Equal(t, closableBuf, spyPushToWriter.args.writer)
				assert.Equal(t, spyOpenWriteCloser.args.path, p.secretGroups[0].FilePath)
				assert.Nil(t, err)
			},
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.description, func(t *testing.T) {
			// Setup mocks
			closableBuf := new(ClosableBuffer)
			spyPushToWriter := pushToWriterSpy{}
			spyOpenWriteCloser := openWriteCloserSpy{
				writeCloser: closableBuf,
			}

			err := provideWithDeps(
				tc.provider.secretGroups,
				tc.provider.retrieveSecretsFunc,
				spyOpenWriteCloser.Call,
				spyPushToWriter.Call,
			)

			tc.assert(t, tc.provider, err, closableBuf, spyPushToWriter, spyOpenWriteCloser)
		})
	}
}
