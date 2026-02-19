package k8ssecretsstorage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	conjurMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur/mocks"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	filetemplates "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/file_templates"
	k8sStorageMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage/mocks"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

var testConjurSecrets = map[string]string{
	"conjur/var/path1":        "secret-value1",
	"conjur/var/path2":        "secret-value2",
	"conjur/var/path3":        "secret-value3",
	"conjur/var/path4":        "secret-value4",
	"conjur/var/umlaut":       "ÄäÖöÜü",
	"conjur/var/binary":       "\xf0\xff\x4a\xc3",
	"conjur/var/empty-secret": "",
	"conjur/var/encoded1":     "ZGVjb2RlZC12YWx1ZS0x", // == decoded-value-1
	"conjur/var/encoded2":     "ZGVjb2RlZC12YWx1ZS0y", // == decoded-value-2
	"conjur/var/encoded3":     "ZGVjb2RlZC12YWx1ZS0z", // == decoded-value-3
}

type testMocks struct {
	conjurClient *conjurMocks.ConjurMockClient
	kubeClient   *k8sStorageMocks.KubeSecretsClient
	logger       *k8sStorageMocks.Logger
}

func newTestMocks() testMocks {
	mocks := testMocks{
		conjurClient: conjurMocks.NewConjurMockClient(),
		kubeClient:   k8sStorageMocks.NewKubeSecretsClient(),
		logger:       k8sStorageMocks.NewLogger(),
	}
	// Populate Conjur with some test secrets
	mocks.conjurClient.AddSecrets(testConjurSecrets)
	return mocks
}

func (m testMocks) setPermissions(denyConjurRetrieve, denyK8sRetrieve,
	denyK8sUpdate bool) {
	if denyConjurRetrieve {
		m.conjurClient.ErrOnExecute = errors.New("custom error")
	}
	if denyK8sRetrieve {
		m.kubeClient.ErrOnRetrieve = errors.New("custom error")
	}
	if denyK8sUpdate {
		m.kubeClient.ErrOnUpdate = errors.New("custom error")
	} else {
		m.kubeClient.ErrOnUpdate = nil
	}
}

func (m testMocks) newProvider(requiredSecrets []string) K8sProvider {
	return newProvider(
		k8sProviderDeps{
			k8s: k8sAccessDeps{
				m.kubeClient.RetrieveSecret,
				m.kubeClient.UpdateSecret,
				m.kubeClient.ListSecrets,
			},
			conjur: conjurAccessDeps{
				m.conjurClient.RetrieveSecrets,
			},
			log: logDeps{
				m.logger.RecordedError,
				m.logger.Error,
				m.logger.Warn,
				m.logger.Info,
				m.logger.Debug,
			},
		},
		true,
		K8sProviderConfig{
			RequiredK8sSecrets: requiredSecrets,
			PodNamespace:       "someNamespace",
		},
		context.Background())
}

type assertFunc func(*testing.T, testMocks, bool, error, string)
type expectedK8sSecrets map[string]map[string]string
type expectedMissingValues map[string][]string

func assertErrorContains(expErrStr string, expectUpdated bool) assertFunc {
	return func(t *testing.T, _ testMocks,
		updated bool, err error, desc string) {

		assert.Error(t, err, desc)
		assert.Contains(t, err.Error(), expErrStr, desc)
		assert.Equal(t, expectUpdated, updated, desc)
	}
}

func assertSecretsUpdated(expK8sSecrets expectedK8sSecrets,
	expMissingValues expectedMissingValues, expectError bool) assertFunc {
	return func(t *testing.T, mocks testMocks, updated bool,
		err error, desc string) {

		if expectError {
			assert.Error(t, err, desc)
		} else {
			assert.NoError(t, err, desc)
			assert.True(t, updated, desc)
		}

		// Check that K8s Secrets contain expected Conjur secret values
		for k8sSecretName, expSecretData := range expK8sSecrets {
			actualSecretData := mocks.kubeClient.InspectSecret(k8sSecretName)
			for secretName, expSecretValue := range expSecretData {
				newDesc := desc + ", Secret: " + secretName
				actualSecretValue := string(actualSecretData[secretName])
				assert.Equal(t, expSecretValue, actualSecretValue, newDesc)
			}
		}
		// Check for secret values leaking into the wrong K8s Secrets
		for k8sSecretName, expMissingValuesForSecret := range expMissingValues {
			actualSecretData := mocks.kubeClient.InspectSecret(k8sSecretName)
			for _, value := range actualSecretData {
				actualValue := string(value)
				newDesc := desc + ", Leaked secret value: " + actualValue
				for _, expMissingValue := range expMissingValuesForSecret {
					assert.NotContains(t, actualValue, expMissingValue, newDesc)
				}
			}
		}
	}
}

func assertErrorLogged(msg string, args ...interface{}) assertFunc {
	return func(t *testing.T, mocks testMocks, updated bool, err error, desc string) {
		errStr := fmt.Sprintf(msg, args...)
		newDesc := desc + ", error logged: " + errStr
		assert.True(t, mocks.logger.ErrorWasLogged(errStr), newDesc)
	}
}

func assertNoErrorLogged() assertFunc {
	return func(t *testing.T, mocks testMocks, updated bool, err error, desc string) {
		// Check that no errors were logged by seeing if ErrorWasLogged returns false for any common error
		assert.False(t, mocks.logger.ErrorWasLogged(""), desc+": should have no error logs")
	}
}

func assertLogged(expected bool, logLevel string, msg string, args ...interface{}) assertFunc {
	return func(t *testing.T, mocks testMocks, updated bool, err error, desc string) {
		logStr := fmt.Sprintf(msg, args...)
		var logDesc string
		if expected {
			logDesc = fmt.Sprintf(", expected %s log to contain: ", logLevel)
		} else {
			logDesc = fmt.Sprintf(", expected  %s log NOT to contain: ", logLevel)
		}
		newDesc := desc + logDesc + logStr
		switch logLevel {
		case "debug":
			assert.Equal(t, expected, mocks.logger.DebugWasLogged(logStr), newDesc)
		case "info":
			assert.Equal(t, expected, mocks.logger.InfoWasLogged(logStr), newDesc)
		case "warn":
			assert.Equal(t, expected, mocks.logger.WarningWasLogged(logStr), newDesc)
		case "error":
			assert.Equal(t, expected, mocks.logger.ErrorWasLogged(logStr), newDesc)
		default:
			assert.Fail(t, "Invalid log level: "+logLevel)
		}
	}
}

func TestProvide(t *testing.T) {
	testCases := []struct {
		desc                   string
		k8sSecrets             k8sStorageMocks.K8sSecrets
		annotations            map[string]map[string]string
		requiredSecrets        []string
		denyConjurRetrieve     bool
		denyK8sRetrieve        bool
		denyK8sUpdate          bool
		alternateConjurSecrets map[string]string
		asserts                []assertFunc
	}{
		{
			desc: "Happy path, existing k8s Secret with existing Conjur secret",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "secret-value1"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Happy path group template, existing k8s Secret with existing Conjur secret",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.secret2":       "- var: conjur/var/path2",
					"conjur.org/secret-file-template.secret2": "{{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "secret-value1", "secret2": "secret-value2"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Happy path, 2 existing k8s Secrets with 2 existing Conjur secrets",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "conjur/var/path2",
					},
				},
				"k8s-secret2": {
					"conjur-map": {
						"secret3": "conjur/var/path3",
						"secret4": "conjur/var/path4",
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1", "k8s-secret2"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"secret1": "secret-value1",
							"secret2": "secret-value2",
						},
						"k8s-secret2": {
							"secret3": "secret-value3",
							"secret4": "secret-value4",
						},
					},
					expectedMissingValues{
						"k8s-secret1": {"secret-value3", "secret-value4"},
						"k8s-secret2": {"secret-value1", "secret-value2"},
					},
					false,
				),
			},
		},
		{
			desc: "Happy path without requiredSecrets configured, 2 existing k8s Secrets with 2 existing Conjur secrets",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-labeled-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "conjur/var/path2",
					},
				},
				"k8s-labeled-secret2": {
					"conjur-map": {
						"secret3": "conjur/var/path3",
						"secret4": "conjur/var/path4",
					},
				},
			},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-labeled-secret1": {
							"secret1": "secret-value1",
							"secret2": "secret-value2",
						},
						"k8s-labeled-secret2": {
							"secret3": "secret-value3",
							"secret4": "secret-value4",
						},
					},
					expectedMissingValues{
						"k8s-labeled-secret1": {"secret-value3", "secret-value4"},
						"k8s-labeled-secret2": {"secret-value1", "secret-value2"},
					},
					false,
				),
			},
		},
		{
			desc: "Happy path with group template, 2 existing k8s Secrets with 2 existing Conjur secrets",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
					},
				},
				"k8s-secret2": {
					"conjur-map": {
						"secret3": "conjur/var/path3",
						"secret4": "conjur/var/path4",
					},
				},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.secret2":       "- var: conjur/var/path2",
					"conjur.org/secret-file-template.secret2": "template-secret2: {{ secret \"var\" }}",
				},
				"k8s-secret2": {
					"conjur.org/conjur-secrets.secret3":       "- var: conjur/var/path3",
					"conjur.org/secret-file-template.secret3": "template-secret3: \n{{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1", "k8s-secret2"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"secret1": "secret-value1",
							"secret2": "template-secret2: " +
								"secret-value2",
						},
						"k8s-secret2": {
							"secret3": "template-secret3: " +
								"\nsecret-value3",
							"secret4": "secret-value4",
						},
					},
					expectedMissingValues{
						"k8s-secret1": {"secret-value3", "secret-value4"},
						"k8s-secret2": {"secret-value1", "secret-value2"},
					},
					false,
				),
			},
		},
		{
			desc: "Happy path, 2 k8s Secrets use the same Conjur secret with different names",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "conjur/var/path2",
					},
				},
				"k8s-secret2": {
					"conjur-map": {
						"secret3": "conjur/var/path2",
						"secret4": "conjur/var/path4",
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1", "k8s-secret2"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"secret1": "secret-value1",
							"secret2": "secret-value2",
						},
						"k8s-secret2": {
							"secret3": "secret-value2",
							"secret4": "secret-value4",
						},
					},
					expectedMissingValues{
						"k8s-secret1": {"secret-value4"},
						"k8s-secret2": {"secret-value1"},
					},
					false,
				),
			},
		},
		{
			desc: "Happy path, 2 existing k8s Secrets but only 1 managed by SP",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "conjur/var/path2",
					},
				},
				"k8s-secret2": {
					"conjur-map": {
						"secret2": "conjur/var/path2",
						"secret3": "conjur/var/path3",
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"secret1": "secret-value1",
							"secret2": "secret-value2",
						},
					},
					expectedMissingValues{
						"k8s-secret1": {"secret-value3"},
						"k8s-secret2": {"secret-value1", "secret-value2", "secret-value3"},
					},
					false,
				),
			},
		},
		{
			desc: "Happy path, k8s Secret maps to Conjur secret with null string value",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/empty-secret"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": ""},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Happy path, secret with umlaut characters",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/umlaut"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "ÄäÖöÜü"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Happy path with group template, secret with umlaut characters",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.secret1":       "- var: conjur/var/umlaut",
					"conjur.org/secret-file-template.secret1": "this is \n{{ secret \"var\" }} \n data",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "this is \nÄäÖöÜü \n data"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Happy path, binary secret",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/binary"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "\xf0\xff\x4a\xc3"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Happy path with template group, binary secret",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.secret1":       "- var: conjur/var/binary",
					"conjur.org/secret-file-template.secret1": "this is binary: {{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "this is binary: \xf0\xff\x4a\xc3"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Happy path, encoded secrets with valid content-type",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"test-decoding": map[string]interface{}{
							"id":           "conjur/var/encoded1",
							"content-type": "base64",
						},
						"test-decoding2": map[string]interface{}{
							"id":           "conjur/var/encoded2",
							"content-type": "base64",
						},
					},
				},
				"k8s-secret2": {
					"conjur-map": {
						"test-still-encoded": "conjur/var/encoded1",
						"test-still-encoded2": map[string]interface{}{
							"id":           "conjur/var/encoded2",
							"content-type": "text",
						},
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1", "k8s-secret2"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"test-decoding":  "decoded-value-1",
							"test-decoding2": "decoded-value-2",
						},
						"k8s-secret2": {
							"test-still-encoded":  "ZGVjb2RlZC12YWx1ZS0x",
							"test-still-encoded2": "ZGVjb2RlZC12YWx1ZS0y",
						},
					},
					expectedMissingValues{},
					false,
				),
				assertLogged(true, "info", messages.CSPFK022I, "test-decoding", "k8s-secret1"),
				assertLogged(true, "info", messages.CSPFK022I, "test-decoding2", "k8s-secret1"),
				assertLogged(false, "info", messages.CSPFK022I, "test-still-encoded", "k8s-secret2"),
				assertLogged(false, "info", messages.CSPFK022I, "test-still-encoded2", "k8s-secret2"),
			},
		},
		{
			desc: "Happy path with template group, encoded secrets with valid content-type",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"test-decoding": map[string]interface{}{
							"id":           "conjur/var/encoded1",
							"content-type": "base64",
						},
					},
				},
				"k8s-secret2": {
					"conjur-map": {
						"test-still-encoded": "conjur/var/encoded1",
					},
				},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.test-decoding2":       "- var: conjur/var/encoded2",
					"conjur.org/secret-file-template.test-decoding2": "this is decoded base64: {{ secret \"var\" | b64dec }}",
				},
				"k8s-secret2": {
					"conjur.org/conjur-secrets.test-still-encoded2":       "- var: conjur/var/encoded2",
					"conjur.org/secret-file-template.test-still-encoded2": "this is encoded base64: {{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1", "k8s-secret2"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"test-decoding":  "decoded-value-1",
							"test-decoding2": "this is decoded base64: decoded-value-2",
						},
						"k8s-secret2": {
							"test-still-encoded":  "ZGVjb2RlZC12YWx1ZS0x",
							"test-still-encoded2": "this is encoded base64: ZGVjb2RlZC12YWx1ZS0y",
						},
					},
					expectedMissingValues{},
					false,
				),
				assertLogged(true, "info", messages.CSPFK022I, "test-decoding", "k8s-secret1"),
				assertLogged(false, "info", messages.CSPFK022I, "test-still-encoded", "k8s-secret2"),
				assertLogged(false, "info", messages.CSPFK022I, "test-still-encoded2", "k8s-secret2"),
			},
		},
		{
			desc: "Plaintext secret with base64 content-type returns the secret as is",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"secret1": map[string]interface{}{
							"id":           "conjur/var/path1",
							"content-type": "base64",
						},
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"secret1": "secret-value1",
						},
					},
					expectedMissingValues{},
					false,
				),
				assertLogged(true, "warn", messages.CSPFK064E, "secret1", "base64", "illegal base64 data"),
			},
		},
		{
			desc: "Invalid or empty content-type is treated as text",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"test-decoding": map[string]interface{}{
							"id":           "conjur/var/encoded1",
							"content-type": "gibberish",
						},
						"test-decoding2": map[string]interface{}{
							"id":           "conjur/var/encoded2",
							"content-type": "",
						},
						"test-decoding3": map[string]interface{}{
							"id": "conjur/var/encoded3",
						},
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"test-decoding":  "ZGVjb2RlZC12YWx1ZS0x",
							"test-decoding2": "ZGVjb2RlZC12YWx1ZS0y",
							"test-decoding3": "ZGVjb2RlZC12YWx1ZS0z",
						},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "Empty var ID throws error",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"test-decoding": map[string]interface{}{
							"id":           "",
							"content-type": "text",
						},
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK037E, "test-decoding", "k8s-secret1"),
			},
		},
		{
			desc: "Empty var ID in template group throws error",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"test-decoding": map[string]interface{}{
							"id":           "",
							"content-type": "text",
						},
					},
				},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.test-decoding":       "- var: \"\"",
					"conjur.org/secret-file-template.test-decoding": "{{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK037E, "test-decoding", "k8s-secret1"),
			},
		},
		{
			desc: "Missing var ID throws error",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"test-decoding": map[string]interface{}{
							"content-type": "text",
						},
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK037E, "test-decoding", "k8s-secret1"),
			},
		},
		{
			desc: "K8s Secrets maps to a non-existent Conjur secret",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "nonexistent/conjur/var"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			denyK8sRetrieve: true,
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK020E),
				assertErrorContains(messages.CSPFK021E, false),
			},
		},
		{
			desc: "Read access to K8s Secrets is not permitted",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			denyK8sRetrieve: true,
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK020E),
				assertErrorContains(messages.CSPFK021E, false),
			},
		},
		{
			desc: "List access to labeled K8s Secrets is not permitted (label-based mode)",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-labeled-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets: []string{}, // Empty list triggers label-based mode
			denyK8sRetrieve: true,
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK024E),
				assertErrorContains(messages.CSPFK021E, false),
			},
		},
		{
			desc: "Access to Conjur secrets is not authorized",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets:    []string{"k8s-secret1"},
			denyConjurRetrieve: true,
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK034E, "custom error"),
				assertErrorContains(fmt.Sprintf(messages.CSPFK034E, "custom error"), false),
			},
		},
		{
			desc: "Updates to K8s 'Secrets' are not permitted",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			denyK8sUpdate:   true,
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK022E),
				assertErrorContains(messages.CSPFK023E, false),
			},
		},
		{
			desc: "K8s secret is required but does not exist",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets: []string{"non-existent-k8s-secret"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK020E),
				assertErrorContains(messages.CSPFK021E, false),
			},
		},
		{
			desc: "K8s secret has no 'conjur-map' entry",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"foobar": {"foo": "bar"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK028E, "k8s-secret1"),
				assertErrorContains(messages.CSPFK021E, false),
			},
		},
		{
			desc: "K8s secret has no 'conjur-map' entry or group template",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"foobar": {"foo": "bar"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK028E, "k8s-secret1"),
				assertErrorContains(messages.CSPFK021E, false),
				assertErrorLogged("At least one of %s data entry or %s annotations must be defined", "conjur-map", "conjur.org/conjur-secrets.* & conjur.org/secret-file-template.*"),
			},
		},
		{
			desc: "K8s secret has an empty 'conjur-map' entry",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK028E, "k8s-secret1"),
				assertErrorContains(messages.CSPFK021E, false),
			},
		},
		{
			desc: "K8s secret has secret group annotation without template annotation, conjur-map still processed",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.group1": "- var: conjur/var/path2",
					// Missing: "conjur.org/secret-file-template.group1"
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "secret-value1"},
					},
					expectedMissingValues{},
					false,
				),
				assertLogged(true, "warn", messages.CSPFK013D, "k8s-secret1", "conjur.org/secret-file-template.group1"),
			},
		},
		{
			desc: "K8s secret has no conjur-map but valid secret groups",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.group1":       "- var: conjur/var/path1",
					"conjur.org/secret-file-template.group1": "{{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"group1": "secret-value1"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "K8s secret has empty conjur-map but valid secret groups",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {},
				},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.group1":       "- var: conjur/var/path1",
					"conjur.org/secret-file-template.group1": "{{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"group1": "secret-value1"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "K8s secret has empty conjur-map and no secret groups",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK028E, "k8s-secret1"),
				assertErrorContains(messages.CSPFK021E, false),
				assertErrorLogged("At least one of %s data entry or %s annotations must be defined", "conjur-map", "conjur.org/conjur-secrets.* & conjur.org/secret-file-template.*"),
			},
		},
		{
			desc: "K8s secret has multiple secret groups, some with templates, some without",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.group1":       "- var: conjur/var/path2",
					"conjur.org/secret-file-template.group1": "group1: {{ secret \"var\" }}",
					"conjur.org/conjur-secrets.group2":       "- var: conjur/var/path3",
					// Missing template for group2 - should warn and skip
					"conjur.org/conjur-secrets.group3":       "- var: conjur/var/path4",
					"conjur.org/secret-file-template.group3": "group3: {{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"secret1": "secret-value1",
							"group1":  "group1: secret-value2",
							"group3":  "group3: secret-value4",
						},
					},
					expectedMissingValues{},
					false,
				),
				assertLogged(true, "warn", messages.CSPFK013D, "k8s-secret1", "conjur.org/secret-file-template.group2"),
			},
		},
		{
			desc: "K8s secret has invalid secret specs in annotation",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.group1":       "invalid: yaml: content",
					"conjur.org/secret-file-template.group1": "{{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK021E),
				assertErrorLogged("unable to create secret specs from annotation"),
				assertErrorLogged("conjur.org/conjur-secrets.group1"),
			},
		},
		{
			desc: "K8s secret has empty secret group annotation value",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {},
			},
			annotations: map[string]map[string]string{
				"k8s-secret1": {
					"conjur.org/conjur-secrets.group1":       "",
					"conjur.org/secret-file-template.group1": "{{ secret \"var\" }}",
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK021E),
				assertErrorLogged(`annotation "conjur.org/conjur-secrets.group1" has no secret specs defined`),
			},
		},
		{
			desc: "K8s secret fetch all happy path",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						// Should have all secrets
						"k8s-secret1": {
							"conjur.var.path1":        "secret-value1",
							"conjur.var.path2":        "secret-value2",
							"conjur.var.path3":        "secret-value3",
							"conjur.var.path4":        "secret-value4",
							"conjur.var.umlaut":       "ÄäÖöÜü",
							"conjur.var.binary":       "\xf0\xff\x4a\xc3",
							"conjur.var.empty-secret": "",
							"conjur.var.encoded1":     "ZGVjb2RlZC12YWx1ZS0x",
							"conjur.var.encoded2":     "ZGVjb2RlZC12YWx1ZS0y",
							"conjur.var.encoded3":     "ZGVjb2RlZC12YWx1ZS0z",
						},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "K8s secret fetch all with base64 decoding",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {
						"*": map[string]interface{}{
							"id":           "*",
							"content-type": "base64",
						},
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						// Should have all secrets
						"k8s-secret1": {
							"conjur.var.path1":        "secret-value1",
							"conjur.var.path2":        "secret-value2",
							"conjur.var.path3":        "secret-value3",
							"conjur.var.path4":        "secret-value4",
							"conjur.var.umlaut":       "ÄäÖöÜü",
							"conjur.var.binary":       "\xf0\xff\x4a\xc3",
							"conjur.var.empty-secret": "",
							// These secrets should be decoded from base64
							"conjur.var.encoded1": "decoded-value-1",
							"conjur.var.encoded2": "decoded-value-2",
							"conjur.var.encoded3": "decoded-value-3",
						},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "K8s secret fetch all with base64 and additional explicit secrets",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
				"k8s-secret2": {
					"conjur-map": {
						"*": map[string]interface{}{
							"id":           "*",
							"content-type": "base64",
						},
					},
				},
				"k8s-secret3": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets: []string{"k8s-secret1", "k8s-secret2", "k8s-secret3"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						// Should have all secrets
						"k8s-secret1": {
							"conjur.var.path1":        "secret-value1",
							"conjur.var.path2":        "secret-value2",
							"conjur.var.path3":        "secret-value3",
							"conjur.var.path4":        "secret-value4",
							"conjur.var.umlaut":       "ÄäÖöÜü",
							"conjur.var.binary":       "\xf0\xff\x4a\xc3",
							"conjur.var.empty-secret": "",
							"conjur.var.encoded1":     "ZGVjb2RlZC12YWx1ZS0x",
							"conjur.var.encoded2":     "ZGVjb2RlZC12YWx1ZS0y",
							"conjur.var.encoded3":     "ZGVjb2RlZC12YWx1ZS0z",
						},
						"k8s-secret2": {
							"conjur.var.path1":        "secret-value1",
							"conjur.var.path2":        "secret-value2",
							"conjur.var.path3":        "secret-value3",
							"conjur.var.path4":        "secret-value4",
							"conjur.var.umlaut":       "ÄäÖöÜü",
							"conjur.var.binary":       "\xf0\xff\x4a\xc3",
							"conjur.var.empty-secret": "",
							// These secrets should be decoded from base64
							"conjur.var.encoded1": "decoded-value-1",
							"conjur.var.encoded2": "decoded-value-2",
							"conjur.var.encoded3": "decoded-value-3",
						},
						// Should have the explicit secret
						"k8s-secret3": {"secret1": "secret-value1"},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "K8s secret fetch all and explicit secret that doesn't exist",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
				"k8s-secret2": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "does/not/exist",
					},
				},
			},
			requiredSecrets: []string{"k8s-secret1", "k8s-secret2"},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						// Should have all secrets
						"k8s-secret1": {
							"conjur.var.path1":        "secret-value1",
							"conjur.var.path2":        "secret-value2",
							"conjur.var.path3":        "secret-value3",
							"conjur.var.path4":        "secret-value4",
							"conjur.var.umlaut":       "ÄäÖöÜü",
							"conjur.var.binary":       "\xf0\xff\x4a\xc3",
							"conjur.var.empty-secret": "",
							"conjur.var.encoded1":     "ZGVjb2RlZC12YWx1ZS0x",
							"conjur.var.encoded2":     "ZGVjb2RlZC12YWx1ZS0y",
							"conjur.var.encoded3":     "ZGVjb2RlZC12YWx1ZS0z",
						},
						// Should have just the explicit secret that does exist
						"k8s-secret2": {
							"secret1": "secret-value1",
							"secret2": "",
						},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "K8s secret fetch all with special characters in secret names",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			alternateConjurSecrets: map[string]string{
				"conjur/var with spaces":       "secret",
				"conjur/var.with.dots":         "secret",
				"conjur/var+with&some*symbols": "secret",
			},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {
							"conjur.var.with.spaces":       "secret",
							"conjur.var.with.dots":         "secret",
							"conjur.var.with.some.symbols": "secret",
						},
					},
					expectedMissingValues{},
					false,
				),
			},
		},
		{
			desc: "K8s secret fetch all with duplicate normalized secret names",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
			},
			requiredSecrets: []string{"k8s-secret1"},
			alternateConjurSecrets: map[string]string{
				"conjur/var 1": "secret1", // normalized to conjur.var.1
				"conjur/var+1": "secret2", // also normalized to conjur.var.1
			},
			asserts: []assertFunc{
				assertLogged(true, "warn", messages.CSPFK067E, "conjur.var.1"),
				// We can't predict which secret will be written to the K8s secret
				// since the order of the secrets is not guaranteed
			},
		},
		{
			desc: "K8s secret fetch all with no secrets in Conjur",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
			},
			requiredSecrets:        []string{"k8s-secret1"},
			alternateConjurSecrets: map[string]string{},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK034E, "no variables to retrieve"),
				assertErrorContains(fmt.Sprintf(messages.CSPFK034E, "no variables to retrieve"), false),
			},
		},
		{
			desc:            "Label-based mode with no pre-configured secrets returns gracefully",
			k8sSecrets:      k8sStorageMocks.K8sSecrets{},
			requiredSecrets: []string{},
			asserts: []assertFunc{
				assertNoErrorLogged(),
				func(t *testing.T, mocks testMocks, updated bool, err error, desc string) {
					assert.NoError(t, err, desc+": should not error")
					assert.False(t, updated, desc+": should not have updates")
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Set up test case
			mocks := newTestMocks()

			if tc.alternateConjurSecrets != nil {
				mocks.conjurClient.ClearSecrets()
				mocks.conjurClient.AddSecrets(tc.alternateConjurSecrets)
			}

			mocks.setPermissions(tc.denyConjurRetrieve, tc.denyK8sRetrieve,
				tc.denyK8sUpdate)

			for secretName, secretData := range tc.k8sSecrets {
				mocks.kubeClient.AddSecret(secretName, tc.annotations[secretName], secretData)
			}
			provider := mocks.newProvider(tc.requiredSecrets)

			// Run test case
			updated, err := provider.Provide()

			// Confirm results
			for _, assert := range tc.asserts {
				assert(t, mocks, updated, err, tc.desc)
			}
		})
	}
}

func TestSecretsContentChanges(t *testing.T) {

	var desc string
	var k8sSecrets k8sStorageMocks.K8sSecrets
	var requiredSecrets []string
	var denyConjurRetrieve bool
	var denyK8sRetrieve bool
	var denyK8sUpdate bool

	// Initial case, k8s secret should be updated
	desc = "Only update secrets when there are changes"
	k8sSecrets = k8sStorageMocks.K8sSecrets{
		"k8s-secret1": {
			"conjur-map": {"secret1": "conjur/var/path1"},
		},
	}
	requiredSecrets = []string{"k8s-secret1"}
	mocks := newTestMocks()
	mocks.setPermissions(denyConjurRetrieve, denyK8sRetrieve, denyK8sUpdate)
	for secretName, secretData := range k8sSecrets {
		mocks.kubeClient.AddSecret(secretName, map[string]string{}, secretData)
	}
	provider := mocks.newProvider(requiredSecrets)
	update, err := provider.Provide()
	assert.False(t, mocks.logger.DebugWasLogged("CSPFK016D"))
	assertSecretsUpdated(
		expectedK8sSecrets{
			"k8s-secret1": {"secret1": "secret-value1"},
		},
		expectedMissingValues{}, false)(t, mocks, update, err, desc)

	// Call Provide again, verify it doesn't try to update the secret
	// as there should be an error if it tried to write the secrets
	desc = "Verify secrets are not updated when there are no changes"
	denyK8sUpdate = true
	mocks.setPermissions(denyConjurRetrieve, denyK8sRetrieve, denyK8sUpdate)
	update, err = provider.Provide()
	assert.NoError(t, err)
	assert.True(t, mocks.logger.DebugWasLogged("CSPFK016D"))
	// verify the same secret still exists
	assertSecretsUpdated(
		expectedK8sSecrets{
			"k8s-secret1": {"secret1": "secret-value1"},
		},
		expectedMissingValues{}, false)(t, mocks, true, err, desc)

	// Change the k8s secret and verify a new secret is written
	desc = "Verify new secrets are written when there are changes to the Conjur secret"
	mocks.logger.ClearInfo()
	mocks.logger.ClearDebug()
	secrets, _ := mocks.kubeClient.RetrieveSecret("", "k8s-secret1")
	var newMap = map[string][]byte{
		"conjur-map": []byte("secret2: conjur/var/path2"),
	}
	denyK8sUpdate = false
	mocks.setPermissions(denyConjurRetrieve, denyK8sRetrieve, denyK8sUpdate)
	mocks.kubeClient.UpdateSecret("mock namespace", "k8s-secret1", secrets, newMap)
	update, err = provider.Provide()
	assert.NoError(t, err)
	assertSecretsUpdated(
		expectedK8sSecrets{
			"k8s-secret1": {"secret2": "secret-value2"},
		},
		expectedMissingValues{}, false)(t, mocks, update, err, desc)
	assert.False(t, mocks.logger.DebugWasLogged("CSPFK016D"))

	// call again with no changes
	desc = "Verify again secrets are not updated when there are no changes"
	update, err = provider.Provide()
	assert.NoError(t, err)
	assert.True(t, mocks.logger.DebugWasLogged("CSPFK016D"))

	// verify a new k8s secret is written when the Conjur secret changes
	desc = "Verify new secrets are written when there are changes to the k8s secret"
	mocks.logger.ClearInfo()
	mocks.logger.ClearDebug()
	var updateConjurSecrets = map[string]string{
		"conjur/var/path1":        "new-secret-value1",
		"conjur/var/path2":        "new-secret-value2",
		"conjur/var/path3":        "new-secret-value3",
		"conjur/var/path4":        "new-secret-value4",
		"conjur/var/empty-secret": "",
	}
	mocks.conjurClient.AddSecrets(updateConjurSecrets)
	update, err = provider.Provide()
	assert.False(t, mocks.logger.DebugWasLogged("CSPFK016D"))
	assertSecretsUpdated(
		expectedK8sSecrets{
			"k8s-secret1": {"secret2": "new-secret-value2"},
		},
		expectedMissingValues{}, false)(t, mocks, update, err, desc)
}

func TestProvideSanitization(t *testing.T) {
	testCases := []struct {
		desc             string
		k8sSecrets       k8sStorageMocks.K8sSecrets
		requiredSecrets  []string
		sanitizeEnabled  bool
		retrieveErrorMsg string
		deleteSecrets    []string
		asserts          []assertFunc
	}{
		{
			desc:            "403 error",
			sanitizeEnabled: true,
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets:  []string{"k8s-secret1"},
			retrieveErrorMsg: "403",
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK034E, "403"),
				assertErrorContains(fmt.Sprintf(messages.CSPFK034E, "403"), true),
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": ""},
					},
					expectedMissingValues{},
					true,
				),
			},
		},
		{
			desc:            "404 error",
			sanitizeEnabled: true,
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets:  []string{"k8s-secret1"},
			retrieveErrorMsg: "404",
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK034E, "404"),
				assertErrorContains(fmt.Sprintf(messages.CSPFK034E, "404"), true),
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": ""},
					},
					expectedMissingValues{},
					true,
				),
			},
		},
		{
			desc:            "generic error doesn't delete secret",
			sanitizeEnabled: true,
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets:  []string{"k8s-secret1"},
			retrieveErrorMsg: "generic error",
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK034E, "generic error"),
				assertErrorContains(fmt.Sprintf(messages.CSPFK034E, "generic error"), false),
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "secret-value1"},
					},
					expectedMissingValues{},
					true,
				),
			},
		},
		{
			desc:            "403 error with sanitize disabled",
			sanitizeEnabled: false,
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"secret1": "conjur/var/path1"},
				},
			},
			requiredSecrets:  []string{"k8s-secret1"},
			retrieveErrorMsg: "403",
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK034E, "403"),
				assertErrorContains(fmt.Sprintf(messages.CSPFK034E, "403"), false),
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-secret1": {"secret1": "secret-value1"},
					},
					expectedMissingValues{},
					true,
				),
			},
		},
		// This test is for a unique edge case: when secrets provider is configured with
		// both a Fetch All group and also a specific secret, and the specific secret is
		// removed from Conjur. The Fetch All group should still be updated, but the
		// specific secret should be removed from the K8s secret IF sanitize is enabled.
		{
			desc:            "secret removed with sanitize enabled and Fetch All group",
			sanitizeEnabled: true,
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
				"k8s-secret2": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "conjur/var/path2",
					},
				},
			},
			requiredSecrets:  []string{"k8s-secret1", "k8s-secret2"},
			retrieveErrorMsg: "",                           // No error, since we're using fetch all
			deleteSecrets:    []string{"conjur/var/path2"}, // Remove a secret
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						// Should have all secrets except the deleted one
						"k8s-secret1": {
							"conjur.var.path1":        "secret-value1",
							"conjur.var.path3":        "secret-value3",
							"conjur.var.path4":        "secret-value4",
							"conjur.var.umlaut":       "ÄäÖöÜü",
							"conjur.var.binary":       "\xf0\xff\x4a\xc3",
							"conjur.var.empty-secret": "",
							"conjur.var.encoded1":     "ZGVjb2RlZC12YWx1ZS0x",
							"conjur.var.encoded2":     "ZGVjb2RlZC12YWx1ZS0y",
							"conjur.var.encoded3":     "ZGVjb2RlZC12YWx1ZS0z",
						},
						// Should have just the explicit secret that still exists
						"k8s-secret2": {
							"secret1": "secret-value1",
							"secret2": "",
						},
					},
					expectedMissingValues{
						"k8s-secret1": {"secret-value2"},
						"k8s-secret2": {"secret-value2"},
					},
					false,
				),
				// Should not get duplicate secret name warning. This would happen
				// if the update destinations are not cleared after each run.
				assertLogged(false, "warn", messages.CSPFK067E, "conjur.var.path1"),
			},
		},
		{
			desc:            "secret removed with sanitize disabled and Fetch All group",
			sanitizeEnabled: false,
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-secret1": {
					"conjur-map": {"*": "*"},
				},
				"k8s-secret2": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "conjur/var/path2",
					},
				},
			},
			requiredSecrets:  []string{"k8s-secret1", "k8s-secret2"},
			retrieveErrorMsg: "",                           // No error, since we're using fetch all
			deleteSecrets:    []string{"conjur/var/path2"}, // Remove a secret
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						// Should have all secrets, even the deleted one
						"k8s-secret1": {
							"conjur.var.path1":        "secret-value1",
							"conjur.var.path2":        "secret-value2",
							"conjur.var.path3":        "secret-value3",
							"conjur.var.path4":        "secret-value4",
							"conjur.var.umlaut":       "ÄäÖöÜü",
							"conjur.var.binary":       "\xf0\xff\x4a\xc3",
							"conjur.var.empty-secret": "",
							"conjur.var.encoded1":     "ZGVjb2RlZC12YWx1ZS0x",
							"conjur.var.encoded2":     "ZGVjb2RlZC12YWx1ZS0y",
							"conjur.var.encoded3":     "ZGVjb2RlZC12YWx1ZS0z",
						},
						// Should still have the deleted secret
						"k8s-secret2": {
							"secret1": "secret-value1",
							"secret2": "secret-value2",
						},
					},
					expectedMissingValues{},
					false,
				),
				// Should not get duplicate secret name warning. This would happen
				// if the update destinations are not cleared after each run.
				assertLogged(false, "warn", messages.CSPFK067E, "conjur.var.path1"),
			},
		},
	}

	for _, tc := range testCases {
		// Set up test case
		mocks := newTestMocks()

		// First do a clean run will all permissions allowed to retrieve and populate the K8s secrets
		provider := mocks.newProvider(tc.requiredSecrets)
		provider.sanitizeEnabled = tc.sanitizeEnabled
		for secretName, secretData := range tc.k8sSecrets {
			mocks.kubeClient.AddSecret(secretName, map[string]string{}, secretData)
		}
		updated, err := provider.Provide()
		assert.NoError(t, err, tc.desc)
		assert.True(t, updated)

		// Now run test case, injecting an error into the retrieve function
		// and removing any secrets that need to be deleted (for the fetch all)
		if tc.retrieveErrorMsg != "" {
			mocks.conjurClient.ErrOnExecute = errors.New(tc.retrieveErrorMsg)
		}
		if len(tc.deleteSecrets) > 0 {
			for _, secretName := range tc.deleteSecrets {
				delete(mocks.conjurClient.Database, secretName)
			}
		}
		updated, err = provider.Provide()

		// Confirm results
		for _, assert := range tc.asserts {
			assert(t, mocks, updated, err, tc.desc)
		}
	}
}

func TestProvideWithCleanup(t *testing.T) {
	testCases := []struct {
		desc            string
		sanitizeEnabled bool
		k8sSecrets      k8sStorageMocks.K8sSecrets
		keysToRemove    map[string][]string
		asserts         []assertFunc
	}{
		{
			desc: "Informer scenario: Remove keys from conjur-map",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-labeled-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
					},
					"secret1": {"value": "old-value1"},
					"secret2": {"value": "stale-value2"},
				},
			},
			keysToRemove: map[string][]string{
				"k8s-labeled-secret1": {"secret2"}, // Informer detected secret2 was removed
			},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-labeled-secret1": {
							"secret1": "secret-value1",
						},
					},
					expectedMissingValues{
						"k8s-labeled-secret1": {"secret-value2"},
					},
					false,
				),
			},
		},
		{
			desc: "Informer scenario: Remove multiple keys from one secret",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-labeled-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
					},
					"secret1": {"value": "old-value1"},
					"secret2": {"value": "stale-value2"},
					"secret3": {"value": "stale-value3"},
				},
			},
			keysToRemove: map[string][]string{
				"k8s-labeled-secret1": {"secret2", "secret3"},
			},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-labeled-secret1": {
							"secret1": "secret-value1",
						},
					},
					expectedMissingValues{
						"k8s-labeled-secret1": {"secret-value2", "secret-value3"},
					},
					false,
				),
			},
		},
		{
			desc: "Informer scenario: Remove keys from multiple secrets",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-labeled-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
					},
					"secret1": {"value": "old-value1"},
					"secret2": {"value": "stale-value2"},
				},
				"k8s-labeled-secret2": {
					"conjur-map": {
						"secret3": "conjur/var/path3",
					},
					"secret3": {"value": "old-value3"},
					"secret4": {"value": "stale-value4"},
				},
			},
			keysToRemove: map[string][]string{
				"k8s-labeled-secret1": {"secret2"},
				"k8s-labeled-secret2": {"secret4"},
			},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-labeled-secret1": {
							"secret1": "secret-value1",
						},
						"k8s-labeled-secret2": {
							"secret3": "secret-value3",
						},
					},
					expectedMissingValues{
						"k8s-labeled-secret1": {"secret-value2", "secret-value3"},
						"k8s-labeled-secret2": {"secret-value1", "secret-value4"},
					},
					false,
				),
			},
		},
		{
			desc: "Informer scenario: Empty keysToRemove - no cleanup",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-labeled-secret1": {
					"conjur-map": {
						"secret1": "conjur/var/path1",
						"secret2": "conjur/var/path2",
					},
					"secret1": {"value": "old-value1"},
					"secret2": {"value": "old-value2"},
				},
			},
			keysToRemove: map[string][]string{},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-labeled-secret1": {
							"secret1": "secret-value1",
							"secret2": "secret-value2",
						},
					},
					expectedMissingValues{
						"k8s-labeled-secret1": {"old-value1", "old-value2"},
					},
					false,
				),
			},
		},
		{
			desc: "Informer scenario: conjur-map cleared entirely",
			k8sSecrets: k8sStorageMocks.K8sSecrets{
				"k8s-labeled-secret1": {
					"conjur-map": {},
					"secret1":    {"value": "stale-value1"},
					"secret2":    {"value": "stale-value2"},
				},
			},
			keysToRemove: map[string][]string{
				"k8s-labeled-secret1": {"secret1", "secret2"},
			},
			asserts: []assertFunc{
				assertSecretsUpdated(
					expectedK8sSecrets{
						"k8s-labeled-secret1": {},
					},
					expectedMissingValues{
						"k8s-labeled-secret1": {"secret1", "secret2"},
					},
					false,
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Set up test case
			mocks := newTestMocks()
			mocks.setPermissions(false, false, false)

			for secretName, secretData := range tc.k8sSecrets {
				mocks.kubeClient.AddSecret(secretName, map[string]string{}, secretData)
			}

			// Label-based mode: requiredSecrets must be empty for informer to work
			provider := mocks.newProvider([]string{})

			// Run test case with cleanup (simulating informer trigger)
			updated, err := provider.ProvideWithCleanup(tc.keysToRemove)

			// Confirm results
			for _, assert := range tc.asserts {
				assert(t, mocks, updated, err, tc.desc)
			}
		})
	}
}

func TestBase64PKCS12SecretPreservesTrailingNull(t *testing.T) {
	original := []byte{0xde, 0xad, 0xbe, 0xef, 0x00}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(original)))
	base64.StdEncoding.Encode(encoded, original)

	provider := K8sProvider{
		secretsState: k8sSecretsState{
			updateDestinations: map[string][]updateDestination{
				"pkcs12var": {{
					k8sSecretName: "pkcs12secret",
					secretName:    "pkcs12file",
					contentType:   "base64",
				}},
			},
		},
	}

	conjurSecrets := map[string][]byte{
		"pkcs12var": encoded,
	}

	secretData := provider.createSecretData(conjurSecrets)
	got := secretData["pkcs12secret"]["pkcs12file"]

	assert.Equal(t, original, got, "decoded PKCS#12 secret should match original including trailing null bytes")
}

func TestUpdateRequiredK8sSecretsWithCleanupRemovesKeys(t *testing.T) {
	mocks := newTestMocks()

	// Prepare K8s Secret with stale key
	mocks.kubeClient.AddSecret("k8s-secret1", map[string]string{}, k8sStorageMocks.K8sSecrets{
		"k8s-secret1": {
			"conjur-map": map[string]interface{}{"secret1": "conjur/var/path1"},
			"secret1":    map[string]interface{}{"value": "old-value1"},
			"secret2":    map[string]interface{}{"value": "stale-value"},
		},
	}["k8s-secret1"])

	// Retrieve the full Secret object from mock client
	originalSecret, err := mocks.kubeClient.RetrieveSecret("", "k8s-secret1")
	assert.NoError(t, err, "should retrieve secret from mock client")

	// Directly construct provider with pre-populated state (simulating informer behavior)
	provider := K8sProvider{
		k8s: k8sAccessDeps{
			mocks.kubeClient.RetrieveSecret,
			mocks.kubeClient.UpdateSecret,
			mocks.kubeClient.ListSecrets,
		},
		conjur: conjurAccessDeps{
			mocks.conjurClient.RetrieveSecrets,
		},
		log: logDeps{
			mocks.logger.RecordedError,
			mocks.logger.Error,
			mocks.logger.Warn,
			mocks.logger.Info,
			mocks.logger.Debug,
		},
		podNamespace: "someNamespace",
		secretsState: k8sSecretsState{
			originalK8sSecrets: map[string]*v1.Secret{
				"k8s-secret1": originalSecret,
			},
			updateDestinations: map[string][]updateDestination{
				"conjur/var/path1": {
					{
						k8sSecretName: "k8s-secret1",
						secretName:    "secret1",
						contentType:   "",
					},
				},
			},
		},
		traceContext:         context.Background(),
		prevSecretsChecksums: map[string]utils.Checksum{},
	}

	tracer := trace.NewOtelTracer(otel.Tracer("test"))

	conjurSecrets := map[string][]byte{
		"conjur/var/path1": []byte("new-value1"),
	}
	keysToRemove := map[string][]string{
		"k8s-secret1": {"secret2"},
	}

	updated, err := provider.updateRequiredK8sSecretsWithCleanup(conjurSecrets, tracer, keysToRemove)
	assert.NoError(t, err, "updateRequiredK8sSecretsWithCleanup should succeed")
	assert.True(t, updated, "updateRequiredK8sSecretsWithCleanup should report updates")

	// Verify that the stale key was removed before update
	if assert.NotNil(t, mocks.kubeClient.LastUpdateOriginalSecret, "original secret should be captured") {
		_, exists := mocks.kubeClient.LastUpdateOriginalSecret.Data["secret2"]
		assert.False(t, exists, "stale key 'secret2' should be removed from original secret before K8s update")
	}
}

func TestParseConjurMap(t *testing.T) {
	testCases := []struct {
		name        string
		secret      *v1.Secret
		expectError bool
		expected    map[string]interface{}
	}{
		{
			name:        "nil secret returns empty map",
			secret:      nil,
			expectError: false,
			expected:    map[string]interface{}{},
		},
		{
			name: "secret with nil Data returns empty map",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
				Data:       nil,
			},
			expectError: false,
			expected:    map[string]interface{}{},
		},
		{
			name: "secret without conjur-map key returns empty map",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
				Data: map[string][]byte{
					"other-key": []byte("value"),
				},
			},
			expectError: false,
			expected:    map[string]interface{}{},
		},
		{
			name: "secret with empty conjur-map returns empty map",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte(""),
				},
			},
			expectError: false,
			expected:    map[string]interface{}{},
		},
		{
			name: "valid YAML conjur-map with string values",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
				},
			},
			expectError: false,
			expected: map[string]interface{}{
				"secret1": "conjur/var/path1",
				"secret2": "conjur/var/path2",
			},
		},
		{
			name: "valid YAML conjur-map with map values containing id and content-type",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1:\n  id: conjur/var/path1\n  content-type: base64\nsecret2: conjur/var/path2"),
				},
			},
			expectError: false,
			expected: map[string]interface{}{
				"secret1": map[interface{}]interface{}{
					"id":           "conjur/var/path1",
					"content-type": "base64",
				},
				"secret2": "conjur/var/path2",
			},
		},
		{
			name: "malformed YAML returns empty map (YAML unmarshal error is ignored)",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("invalid: yaml: content: with: too: many: colons:"),
				},
			},
			expectError: false,
			expected:    map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseConjurMap(tc.secret)
			assert.Equal(t, tc.expected, result, "parseConjurMap should return expected map")
		})
	}
}
func TestCreateGroupTemplateSecretData(t *testing.T) {
	testCases := []struct {
		name               string
		secretsGroups      map[string][]*filetemplates.SecretGroup
		originalK8sSecrets map[string]*v1.Secret
		conjurSecrets      map[string][]byte
		initialDataMap     map[string]map[string][]byte
		expectedResult     map[string]map[string][]byte
		expectedErrors     []string
	}{
		{
			name: "successfully renders template with all secrets present",
			secretsGroups: map[string][]*filetemplates.SecretGroup{
				"k8s-secret1": {
					{
						Name: "group1",
						SecretSpecs: []filetemplates.SecretSpec{
							{Alias: "alias1", Path: "conjur/var/path1"},
							{Alias: "alias2", Path: "conjur/var/path2"},
						},
					},
				},
			},
			originalK8sSecrets: map[string]*v1.Secret{
				"k8s-secret1": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"conjur.org/secret-file-template.group1": "{{ secret \"alias1\" }}-{{ secret \"alias2\" }}",
						},
					},
				},
			},
			conjurSecrets: map[string][]byte{
				"conjur/var/path1": []byte("secret-value1"),
				"conjur/var/path2": []byte("secret-value2"),
			},
			initialDataMap: make(map[string]map[string][]byte),
			expectedResult: map[string]map[string][]byte{
				"k8s-secret1": {
					"group1": []byte("secret-value1-secret-value2"),
				},
			},
		},
		{
			name: "handles missing secret gracefully",
			secretsGroups: map[string][]*filetemplates.SecretGroup{
				"k8s-secret1": {
					{
						Name: "group1",
						SecretSpecs: []filetemplates.SecretSpec{
							{Alias: "alias1", Path: "conjur/var/path1"},
							{Alias: "alias2", Path: "conjur/var/missing"},
						},
					},
				},
			},
			originalK8sSecrets: map[string]*v1.Secret{
				"k8s-secret1": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"conjur.org/secret-file-template.group1": "{{ secret \"alias1\" }}-{{ secret \"alias2\" }}",
						},
					},
				},
			},
			conjurSecrets: map[string][]byte{
				"conjur/var/path1": []byte("secret-value1"),
			},
			initialDataMap: make(map[string]map[string][]byte),
			expectedResult: map[string]map[string][]byte{
				"k8s-secret1": {
					"group1": []byte("secret-value1-"),
				},
			},
			expectedErrors: []string{"CSPFK087E"},
		},
		{
			name: "handles invalid template parse error gracefully",
			secretsGroups: map[string][]*filetemplates.SecretGroup{
				"k8s-secret1": {
					{
						Name: "group1",
						SecretSpecs: []filetemplates.SecretSpec{
							{Alias: "alias1", Path: "conjur/var/path1"},
						},
					},
				},
			},
			originalK8sSecrets: map[string]*v1.Secret{
				"k8s-secret1": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"conjur.org/secret-file-template.group1": "{{ invalid template syntax }",
						},
					},
				},
			},
			conjurSecrets: map[string][]byte{
				"conjur/var/path1": []byte("secret-value1"),
			},
			initialDataMap: make(map[string]map[string][]byte),
			expectedResult: map[string]map[string][]byte{},
			expectedErrors: []string{"CSPFK088E"},
		},
		{
			name: "handles multiple groups in same secret",
			secretsGroups: map[string][]*filetemplates.SecretGroup{
				"k8s-secret1": {
					{
						Name: "group1",
						SecretSpecs: []filetemplates.SecretSpec{
							{Alias: "alias1", Path: "conjur/var/path1"},
						},
					},
					{
						Name: "group2",
						SecretSpecs: []filetemplates.SecretSpec{
							{Alias: "alias2", Path: "conjur/var/path2"},
						},
					},
				},
			},
			originalK8sSecrets: map[string]*v1.Secret{
				"k8s-secret1": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"conjur.org/secret-file-template.group1": "{{ secret \"alias1\" }}",
							"conjur.org/secret-file-template.group2": "{{ secret \"alias2\" }}",
						},
					},
				},
			},
			conjurSecrets: map[string][]byte{
				"conjur/var/path1": []byte("secret-value1"),
				"conjur/var/path2": []byte("secret-value2"),
			},
			initialDataMap: make(map[string]map[string][]byte),
			expectedResult: map[string]map[string][]byte{
				"k8s-secret1": {
					"group1": []byte("secret-value1"),
					"group2": []byte("secret-value2"),
				},
			},
		},
		{
			name: "merges with existing newSecretsDataMap",
			secretsGroups: map[string][]*filetemplates.SecretGroup{
				"k8s-secret1": {
					{
						Name: "group1",
						SecretSpecs: []filetemplates.SecretSpec{
							{Alias: "alias1", Path: "conjur/var/path1"},
						},
					},
				},
			},
			originalK8sSecrets: map[string]*v1.Secret{
				"k8s-secret1": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"conjur.org/secret-file-template.group1": "{{ secret \"alias1\" }}",
						},
					},
				},
			},
			conjurSecrets: map[string][]byte{
				"conjur/var/path1": []byte("secret-value1"),
			},
			initialDataMap: map[string]map[string][]byte{
				"k8s-secret1": {
					"existing-key": []byte("existing-value"),
				},
			},
			expectedResult: map[string]map[string][]byte{
				"k8s-secret1": {
					"existing-key": []byte("existing-value"),
					"group1":       []byte("secret-value1"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mocks := newTestMocks()
			provider := mocks.newProvider([]string{})

			provider.secretsGroups = tc.secretsGroups
			provider.secretsState.originalK8sSecrets = tc.originalK8sSecrets

			// Create a copy of initialDataMap to avoid modifying the test case
			newSecretsDataMap := make(map[string]map[string][]byte)
			for k, v := range tc.initialDataMap {
				newSecretsDataMap[k] = make(map[string][]byte)
				for k2, v2 := range v {
					newSecretsDataMap[k][k2] = v2
				}
			}

			err := provider.populateGroupTemplateSecretData(tc.conjurSecrets, newSecretsDataMap)
			assert.NoError(t, err)

			// Verify results
			assert.Equal(t, len(tc.expectedResult), len(newSecretsDataMap), "result map should have expected number of secrets")
			for secretName, expectedData := range tc.expectedResult {
				assert.NotNil(t, newSecretsDataMap[secretName], "secret %s should exist in result", secretName)
				for key, expectedValue := range expectedData {
					assert.Equal(t, expectedValue, newSecretsDataMap[secretName][key], "value for %s.%s should match", secretName, key)
				}
			}

			// Verify error logging
			for _, errCode := range tc.expectedErrors {
				assert.True(t, mocks.logger.ErrorWasLogged(errCode), "should log error %s", errCode)
			}
		})
	}
}

func secretFromConjurMap(name string, m map[string]interface{}) *v1.Secret {
	if m == nil {
		return nil
	}
	var data []byte
	if len(m) > 0 {
		data, _ = yaml.Marshal(m)
	}
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			config.ConjurMapKey: data,
		},
	}
}

var getRemovedAndAddedConjurMapKeysTestCases = []struct {
	name            string
	oldSecret       *v1.Secret
	newSecret       *v1.Secret
	expectedRemoved []string
	expectedAdded   []string
}{
	{
		name:            "both empty maps",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{}),
		expectedRemoved: []string{},
		expectedAdded:   []string{},
	},
	{
		name:            "old map empty, new map has keys - no removed keys",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "value1", "key2": "value2"}),
		expectedRemoved: []string{},
		expectedAdded:   []string{"key1", "key2"},
	},
	{
		name:            "old map has keys, new map empty - all keys removed",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "value1", "key2": "value2"}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{}),
		expectedRemoved: []string{"key1", "key2"},
		expectedAdded:   []string{},
	},
	{
		name:            "same keys in both maps - no removed keys",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "value1", "key2": "value2"}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "new-value1", "key2": "new-value2"}),
		expectedRemoved: []string{},
		expectedAdded:   []string{},
	},
	{
		name:            "one key removed from old map",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "value1", "key3": "value3"}),
		expectedRemoved: []string{"key2"},
		expectedAdded:   []string{},
	},
	{
		name:            "multiple keys removed from old map",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{"secret1": "path1", "secret2": "path2", "secret3": "path3", "secret4": "path4"}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{"secret1": "path1", "secret4": "path4"}),
		expectedRemoved: []string{"secret2", "secret3"},
		expectedAdded:   []string{},
	},
	{
		name:            "removed keys can have different value types",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{"str_key": "string_value", "map_key": map[string]interface{}{"nested": "value"}}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{"str_key": "string_value"}),
		expectedRemoved: []string{"map_key"},
		expectedAdded:   []string{},
	},
	{
		name:            "keys both removed and added",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "v1", "key2": "v2"}),
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{"key2": "v2", "key3": "v3"}),
		expectedRemoved: []string{"key1"},
		expectedAdded:   []string{"key3"},
	},
	{
		name:            "old secret is nil",
		oldSecret:       nil,
		newSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "value1"}),
		expectedRemoved: []string{},
		expectedAdded:   []string{"key1"},
	},
	{
		name:            "new secret is nil",
		oldSecret:       secretFromConjurMap("secret", map[string]interface{}{"key1": "value1"}),
		newSecret:       nil,
		expectedRemoved: []string{"key1"},
		expectedAdded:   []string{},
	},
}

func TestGetRemovedConjurMapKeys(t *testing.T) {
	for _, tc := range getRemovedAndAddedConjurMapKeysTestCases {
		t.Run(tc.name, func(t *testing.T) {
			removed := getRemovedConjurMapKeys(tc.oldSecret, tc.newSecret)
			assert.ElementsMatch(t, tc.expectedRemoved, removed)
		})
	}
}

func TestGetAddedConjurMapKeys(t *testing.T) {
	for _, tc := range getRemovedAndAddedConjurMapKeysTestCases {
		t.Run(tc.name, func(t *testing.T) {
			added := getAddedConjurMapKeys(tc.oldSecret, tc.newSecret)
			assert.ElementsMatch(t, tc.expectedAdded, added)
		})
	}
}

// TestGetRemovedKeys tests the GetRemovedKeys function
func TestGetRemovedKeys(t *testing.T) {
	testCases := []struct {
		name        string
		oldSecret   *v1.Secret
		newSecret   *v1.Secret
		expected    map[string][]string
		description string
	}{
		{
			name:      "both secrets are nil",
			oldSecret: nil,
			newSecret: nil,
			expected:  map[string][]string{},
		},
		{
			name:      "old secret is nil",
			oldSecret: nil,
			newSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "new-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1"),
				},
			},
			expected: map[string][]string{},
		},
		{
			name: "new secret is nil",
			oldSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "old-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1"),
				},
			},
			newSecret: nil,
			expected:  map[string][]string{},
		},
		{
			name: "no keys removed - same secrets",
			oldSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
				},
			},
			newSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
				},
			},
			expected: map[string][]string{},
		},
		{
			name: "one key removed from conjur-map",
			oldSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2\nsecret3: conjur/var/path3"),
				},
			},
			newSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret3: conjur/var/path3"),
				},
			},
			expected: map[string][]string{"my-secret": {"secret2"}},
		},
		{
			name: "multiple keys removed from conjur-map",
			oldSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "app-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2\nsecret3: conjur/var/path3\nsecret4: conjur/var/path4"),
				},
			},
			newSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "app-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret4: conjur/var/path4"),
				},
			},
			expected: map[string][]string{"app-secret": {"secret2", "secret3"}},
		},
		{
			name: "conjur-map removed entirely from new secret",
			oldSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "db-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("username: conjur/db/user\ndatabase: conjur/db/name"),
				},
			},
			newSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "db-secret"},
				Data: map[string][]byte{
					"other-key": []byte("other-value"),
				},
			},
			expected: map[string][]string{"db-secret": {"username", "database"}},
		},
		{
			name: "conjur-map with map values (id and content-type)",
			oldSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "encoded-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1:\n  id: conjur/var/path1\n  content-type: base64\nsecret2: conjur/var/path2"),
				},
			},
			newSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "encoded-secret"},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("secret1:\n  id: conjur/var/path1\n  content-type: base64"),
				},
			},
			expected: map[string][]string{"encoded-secret": {"secret2"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &K8sProvider{sanitizeEnabled: true}
			result := provider.GetRemovedKeys(tc.oldSecret, tc.newSecret)
			// Verify the result map structure
			assert.Equal(t, len(tc.expected), len(result), "GetRemovedKeys should return correct number of secrets")
			// For each expected secret, verify the removed keys
			for secretName, expectedKeys := range tc.expected {
				actualKeys, exists := result[secretName]
				assert.True(t, exists, "GetRemovedKeys should include secret '%s'", secretName)
				assert.ElementsMatch(t, expectedKeys, actualKeys)
			}
		})
	}
}

func TestMergeReAddedKeys(t *testing.T) {
	provider := &K8sProvider{}

	t.Run("re-added key is filtered out", func(t *testing.T) {
		// Event A removed secret2; Event B re-added it before debounce fired
		cumulative := map[string][]string{"my-secret": {"secret2"}}
		oldSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1"),
			},
		}
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
			},
		}
		result := provider.MergeReAddedKeys(cumulative, oldSecret, newSecret)
		assert.Empty(t, result["my-secret"], "re-added key should be filtered out")
	})

	t.Run("non-re-added keys remain", func(t *testing.T) {
		cumulative := map[string][]string{"my-secret": {"secret2", "secret3"}}
		oldSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1"),
			},
		}
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
			},
		}
		result := provider.MergeReAddedKeys(cumulative, oldSecret, newSecret)
		assert.ElementsMatch(t, []string{"secret3"}, result["my-secret"], "only re-added key should be filtered")
	})

	t.Run("no re-added keys leaves cumulative unchanged", func(t *testing.T) {
		cumulative := map[string][]string{"my-secret": {"secret2", "secret3"}}
		oldSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1"),
			},
		}
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1"),
			},
		}
		result := provider.MergeReAddedKeys(cumulative, oldSecret, newSecret)
		assert.ElementsMatch(t, []string{"secret2", "secret3"}, result["my-secret"])
	})

	t.Run("empty cumulative is no-op", func(t *testing.T) {
		cumulative := map[string][]string{}
		oldSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
			},
		}
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
			},
		}
		result := provider.MergeReAddedKeys(cumulative, oldSecret, newSecret)
		assert.Empty(t, result)
	})

	t.Run("nil secrets returns cumulative unchanged", func(t *testing.T) {
		cumulative := map[string][]string{"my-secret": {"secret2"}}
		result := provider.MergeReAddedKeys(cumulative, nil, nil)
		assert.ElementsMatch(t, []string{"secret2"}, result["my-secret"])
	})

	t.Run("oldSecret nil and newSecret non-nil - keys in new conjur-map are filtered out", func(t *testing.T) {
		cumulative := map[string][]string{"my-secret": {"secret1", "secret2", "secret3"}}
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
			},
		}
		result := provider.MergeReAddedKeys(cumulative, nil, newSecret)
		// secret1, secret2 are in new conjur-map - filtered out (they stay)
		// secret3 is not in new conjur-map - remains in cumulative (will be removed)
		assert.ElementsMatch(t, []string{"secret3"}, result["my-secret"])
	})

	t.Run("oldSecret nil and newSecret non-nil - all keys in new conjur-map means cumulative cleared", func(t *testing.T) {
		cumulative := map[string][]string{"my-secret": {"secret1", "secret2"}}
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("secret1: conjur/var/path1\nsecret2: conjur/var/path2"),
			},
		}
		result := provider.MergeReAddedKeys(cumulative, nil, newSecret)
		assert.Empty(t, result["my-secret"], "all keys in new conjur-map should stay")
	})

	t.Run("only filters secret matching event", func(t *testing.T) {
		cumulative := map[string][]string{
			"secret-a": {"key1"},
			"secret-b": {"key2"},
		}
		oldSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "secret-b"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("key2: conjur/var/path2"),
			},
		}
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "secret-b"},
			Data: map[string][]byte{
				config.ConjurMapKey: []byte("key2: conjur/var/path2"),
			},
		}
		result := provider.MergeReAddedKeys(cumulative, oldSecret, newSecret)
		assert.ElementsMatch(t, []string{"key1"}, result["secret-a"], "secret-a should be unchanged")
		assert.ElementsMatch(t, []string{"key2"}, result["secret-b"], "secret-b has no re-adds")
	})
}
