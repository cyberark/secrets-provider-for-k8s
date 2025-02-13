package k8ssecretsstorage

import (
	"context"
	"errors"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	conjurMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur/mocks"
	k8sStorageMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage/mocks"
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
				mocks.kubeClient.AddSecret(secretName, secretData)
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
		mocks.kubeClient.AddSecret(secretName, secretData)
	}
	provider := mocks.newProvider(requiredSecrets)
	update, err := provider.Provide()
	assert.False(t, mocks.logger.InfoWasLogged(messages.CSPFK020I))
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
	assert.True(t, mocks.logger.InfoWasLogged(messages.CSPFK020I))
	// verify the same secret still exists
	assertSecretsUpdated(
		expectedK8sSecrets{
			"k8s-secret1": {"secret1": "secret-value1"},
		},
		expectedMissingValues{}, false)(t, mocks, true, err, desc)

	// Change the k8s secret and verify a new secret is written
	desc = "Verify new secrets are written when there are changes to the Conjur secret"
	mocks.logger.ClearInfo()
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
	assert.False(t, mocks.logger.InfoWasLogged(messages.CSPFK020I))

	// call again with no changes
	desc = "Verify again secrets are not updated when there are no changes"
	update, err = provider.Provide()
	assert.NoError(t, err)
	assert.True(t, mocks.logger.InfoWasLogged(messages.CSPFK020I))

	// verify a new k8s secret is written when the Conjur secret changes
	desc = "Verify new secrets are written when there are changes to the k8s secret"
	mocks.logger.ClearInfo()
	var updateConjurSecrets = map[string]string{
		"conjur/var/path1":        "new-secret-value1",
		"conjur/var/path2":        "new-secret-value2",
		"conjur/var/path3":        "new-secret-value3",
		"conjur/var/path4":        "new-secret-value4",
		"conjur/var/empty-secret": "",
	}
	mocks.conjurClient.AddSecrets(updateConjurSecrets)
	update, err = provider.Provide()
	assert.False(t, mocks.logger.InfoWasLogged(messages.CSPFK020I))
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
			mocks.kubeClient.AddSecret(secretName, secretData)
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

func TestMutate(t *testing.T) {
	testCases := []struct {
		desc                   string
		k8sSecrets             k8sStorageMocks.K8sSecrets
		mutateSecret           v1.Secret
		requiredSecrets        []string
		denyConjurRetrieve     bool
		denyK8sRetrieve        bool
		denyK8sUpdate          bool
		alternateConjurSecrets map[string]string
		expectedSecrets        map[string]string
		asserts                []assertFunc
	}{
		{
			desc: "Happy path, existing Conjur secret",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`"secret1": "conjur/var/path1"`),
				},
			},
			expectedSecrets: map[string]string{
				"secret1": "secret-value1",
			},
		},
		{
			desc: "Happy path, 2 existing Conjur secrets",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`
"secret1": "conjur/var/path1"
"secret2": "conjur/var/path2"`),
				},
			},
			requiredSecrets: []string{"k8s-secret2"}, //configured secrets should be ignored in mutate case
			expectedSecrets: map[string]string{
				"secret1": "secret-value1",
				"secret2": "secret-value2",
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
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`
"secret1":
    "id": "conjur/var/path1"
    "content-type": "base64"`),
				},
			},
			requiredSecrets: []string{"k8s-secret2", "k8s-secret3"}, //configured secrets should be ignored in mutate case
			expectedSecrets: map[string]string{
				"secret1": "secret-value1",
			},
			asserts: []assertFunc{
				assertLogged(true, "warn", messages.CSPFK064E, "secret1", "base64", "illegal base64 data"),
			},
		},
		{
			desc: "Happy path, k8s Secret maps to Conjur secret with null string value",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`"secret1": "conjur/var/empty-secret"`),
				},
			},
			expectedSecrets: map[string]string{
				"secret1": "",
			},
		},
		{
			desc: "Happy path, binary secret",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`"secret1": "conjur/var/binary"`),
				},
			},
			expectedSecrets: map[string]string{
				"secret1": "\xf0\xff\x4a\xc3",
			},
		},
		{
			desc: "Happy path, encoded secrets with valid content-type",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`
"test-decoding":
    "id": "conjur/var/encoded1"
    "content-type": "base64"
"test-still-encoded":
    "id": "conjur/var/encoded1"
    "content-type": "text"
`),
				},
			},
			expectedSecrets: map[string]string{
				"test-decoding":      "decoded-value-1",
				"test-still-encoded": "ZGVjb2RlZC12YWx1ZS0x",
			},
			asserts: []assertFunc{
				assertLogged(true, "info", messages.CSPFK022I, "test-decoding", "k8s-secret1"),
				assertLogged(false, "info", messages.CSPFK022I, "test-still-encoded", "k8s-secret1"),
			},
		},
		{
			desc: "Empty var ID throws error",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`
"test-decoding":
    "id": ""
    "content-type": "test"`),
				},
			},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK037E, "test-decoding", "k8s-secret1"),
			},
		},
		{
			desc: "Access to Conjur secrets is not authorized",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`"secret1": "conjur/var/path1"`),
				},
			},
			denyConjurRetrieve: true,
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK070E, "k8s-secret1", "custom error"),
			},
		},
		{
			desc: "K8s secret has no 'conjur-map' entry",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"foobar": []byte(`"foo": "bar"`),
				},
			},
			expectedSecrets: map[string]string{},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK028E, "k8s-secret1"),
			},
		},
		{
			desc: "K8s secret has an empty 'conjur-map' entry",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(``),
				},
			},
			expectedSecrets: map[string]string{},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK028E, "k8s-secret1"),
			},
		},
		{
			desc: "K8s secret fetch all happy path",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`"*": "*"`),
				},
			},
			expectedSecrets: map[string]string{
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
		{
			desc: "K8s secret fetch all with duplicate normalized secret names",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`"*": "*"`),
				},
			},
			alternateConjurSecrets: map[string]string{
				"conjur/var 1": "secret1", // normalized to conjur.var.1
				"conjur/var+1": "secret2", // also normalized to conjur.var.1
			},
			expectedSecrets: map[string]string{},
			asserts: []assertFunc{
				assertLogged(true, "warn", messages.CSPFK067E, "conjur.var.1"),
				// We can't predict which secret will be written to the K8s secret
				// since the order of the secrets is not guaranteed
			},
		},
		{
			desc: "K8s secret fetch all with no secrets in Conjur",
			mutateSecret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "k8s-secret1",
				},
				Data: map[string][]byte{
					"conjur-map": []byte(`"*": "*"`),
				},
			},
			alternateConjurSecrets: map[string]string{},
			expectedSecrets:        map[string]string{},
			asserts: []assertFunc{
				assertErrorLogged(messages.CSPFK034E, "no variables to retrieve"),
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
				mocks.kubeClient.AddSecret(secretName, secretData)
			}
			provider := mocks.newProvider(tc.requiredSecrets)

			// Run test case
			actualSecretValue, err := provider.Mutate(tc.mutateSecret)

			for key, val := range tc.expectedSecrets {
				assert.Equal(t, val, string(actualSecretValue[key]), "")
			}

			if tc.asserts != nil {
				for _, assert := range tc.asserts {
					assert(t, mocks, true, err, tc.desc)
				}
			}
		})
	}
}
