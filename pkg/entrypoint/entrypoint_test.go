package entrypoint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/stretchr/testify/assert"
)

type mockRetrieverFactory struct {
	retriever conjur.RetrieveSecretsFunc
	err       error
}

func (r mockRetrieverFactory) GetRetriever(a config.Configuration) (conjur.RetrieveSecretsFunc, error) {
	return r.retriever, r.err
}

type mockRetriever struct {
	data map[string][]byte
	err  error
}

func (r mockRetriever) Retrieve(ids []string, c context.Context) (map[string][]byte, error) {
	return r.data, r.err
}

type mockProviderFactory struct {
	providerFunc secrets.ProviderFunc
	errs         []error
}

func (p mockProviderFactory) GetProvider(traceContext context.Context, secretsRetrieverFunc conjur.RetrieveSecretsFunc, providerConfig secrets.ProviderConfig) (secrets.ProviderFunc, []error) {
	return p.providerFunc, p.errs
}

func getMockStatusUpdater() secrets.StatusUpdater {
	return mockStatusUpdater{}
}

type mockStatusUpdater struct{}

func (s mockStatusUpdater) SetSecretsProvided() error {
	return nil
}

func (s mockStatusUpdater) SetSecretsUpdated() error {
	return nil
}

func (s mockStatusUpdater) CopyScripts() error {
	return nil
}

type editableMap map[string]string

func (m editableMap) Delete(key string) editableMap {
	delete(m, key)
	return m
}

func (m editableMap) Edit(key string, value string) editableMap {
	m[key] = value
	return m
}

func (m editableMap) Copy() editableMap {
	mCopy := editableMap{}
	for k, v := range m {
		mCopy[k] = v
	}
	return mCopy
}

func TestStartSecretsProvider(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "entrypoint_test")
	defer os.RemoveAll(tmpDir)

	env := map[string]string{
		"MY_POD_NAME":             "podname",
		"MY_POD_NAMESPACE":        "podnamespace",
		"CONJUR_ACCOUNT":          "default",
		"CONJUR_APPLIANCE_URL":    "https://conjur.myorg.com",
		"CONJUR_AUTHN_URL":        "https://conjur.myorg.com/authn-k8s/authn-service",
		"CONJUR_SSL_CERTIFICATE":  "cert-data",
		"CONJUR_AUTHENTICATOR_ID": "authn-service",
	}
	annots := editableMap{
		"conjur.org/authn-identity":        "host/alice",
		"conjur.org/container-mode":        "init",
		"conjur.org/secrets-destination":   "file",
		"conjur.org/conjur-secrets.groupA": `- alias: path/to/secret\n`,
	}
	annotationsFilePath := filepath.Join(tmpDir, "annotations")
	secretsBasePath := filepath.Join(tmpDir, "secrets")
	templatesBasePath := filepath.Join(tmpDir, "templates")

	TestCases := []struct {
		description      string
		environment      map[string]string
		annotations      map[string]string
		retrieverFactory mockRetrieverFactory
		providerFactory  mockProviderFactory
		assertions       func(*testing.T, int, string)
	}{
		{
			description: "happy path",
			environment: env,
			annotations: annots,
			retrieverFactory: mockRetrieverFactory{
				retriever: mockRetriever{
					data: map[string][]byte{
						"path/to/secret": []byte("secret value"),
					},
					err: nil,
				}.Retrieve,
			},
			providerFactory: mockProviderFactory{
				providerFunc: func() (bool, error) {
					return true, nil
				},
				errs: []error{},
			},
			assertions: func(t *testing.T, code int, logs string) {
				assert.Equal(t, 0, code)
			},
		},
		{
			description: "bad provider factory",
			environment: env,
			annotations: annots,
			retrieverFactory: mockRetrieverFactory{
				retriever: mockRetriever{
					data: map[string][]byte{
						"path/to/secret": []byte("secret value"),
					},
					err: nil,
				}.Retrieve,
			},
			providerFactory: mockProviderFactory{
				providerFunc: func() (bool, error) {
					return true, nil
				},
				errs: []error{errors.New("provider factory failure")},
			},
			assertions: func(t *testing.T, code int, logs string) {
				assert.Equal(t, 1, code)
				assert.Contains(t, logs, "CSPFK053E")
				assert.Contains(t, logs, "provider factory failure")
			},
		},
		{
			description: "bad retriever factory",
			environment: env,
			annotations: annots,
			retrieverFactory: mockRetrieverFactory{
				retriever: mockRetriever{
					data: nil,
					err:  nil,
				}.Retrieve,
				err: errors.New("retriever factory failure"),
			},
			providerFactory: mockProviderFactory{
				providerFunc: func() (bool, error) {
					return true, nil
				},
				errs: []error{},
			},
			assertions: func(t *testing.T, code int, logs string) {
				assert.Equal(t, 1, code)
				assert.Contains(t, logs, "retriever factory failure")
			},
		},
		{
			description: "annotation validation failure",
			environment: env,
			annotations: annots.Copy().Edit("conjur.org/retry-interval-sec", "not-an-integer"),
			retrieverFactory: mockRetrieverFactory{
				retriever: mockRetriever{
					data: nil,
					err:  nil,
				}.Retrieve,
				err: nil,
			},
			providerFactory: mockProviderFactory{
				providerFunc: func() (bool, error) {
					return true, nil
				},
				errs: []error{},
			},
			assertions: func(t *testing.T, code int, logs string) {
				assert.Equal(t, 1, code)
				assert.Contains(t, logs, "CSPFK049E")
			},
		},
		{
			description: "authenticator config validation failure",
			environment: env,
			annotations: annots.Copy().Edit("conjur.org/authn-identity", "1"),
			retrieverFactory: mockRetrieverFactory{
				retriever: mockRetriever{
					data: nil,
					err:  nil,
				}.Retrieve,
				err: nil,
			},
			providerFactory: mockProviderFactory{
				providerFunc: func() (bool, error) {
					return true, nil
				},
				errs: []error{},
			},
			assertions: func(t *testing.T, code int, logs string) {
				assert.Equal(t, 1, code)
				assert.Contains(t, logs, "CSPFK008E")
			},
		},
		{
			description: "secrets provider setting validation failure",
			environment: env,
			annotations: annots.Copy().Delete("conjur.org/secrets-destination"),
			retrieverFactory: mockRetrieverFactory{
				retriever: mockRetriever{
					data: nil,
					err:  nil,
				}.Retrieve,
				err: nil,
			},
			providerFactory: mockProviderFactory{
				providerFunc: func() (bool, error) {
					return true, nil
				},
				errs: []error{},
			},
			assertions: func(t *testing.T, code int, logs string) {
				assert.Equal(t, 1, code)
				assert.Contains(t, logs, "CSPFK015E")
			},
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.description, func(t *testing.T) {
			// Capture error logs
			buf := &bytes.Buffer{}
			log.ErrorLogger.SetOutput(buf)
			// Burn info logs
			log.InfoLogger.SetOutput(&bytes.Buffer{})
			// Setup envvars
			for k, v := range tc.environment {
				os.Setenv(k, v)
			}
			// Setup annotation file
			annotationFileContent := ""
			for k, v := range tc.annotations {
				annotationFileContent = fmt.Sprintf("%s%s=\"%s\"\n", annotationFileContent, k, v)
			}
			err := os.WriteFile(annotationsFilePath, []byte(annotationFileContent), 0666)
			assert.Nil(t, err)

			exitCode := startSecretsProviderWithDeps(
				annotationsFilePath,
				secretsBasePath,
				templatesBasePath,
				tc.retrieverFactory.GetRetriever,
				tc.providerFactory.GetProvider,
				getMockStatusUpdater,
			)

			tc.assertions(t, exitCode, buf.String())

			// Restore logs
			log.ErrorLogger.SetOutput(os.Stderr)
			// Teardown envvars
			for k := range tc.environment {
				os.Setenv(k, "")
			}
		})
	}
}
