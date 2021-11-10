package k8ssecretsstorage

import (
	"fmt"
	"testing"

	//. "github.com/smartystreets/goconvey/convey"
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
	"conjur/var/empty-secret": "",
}

type testMocks struct {
	conjurClient *conjurMocks.ConjurClient
	kubeClient   *k8sStorageMocks.KubeSecretsClient
	logger       *k8sStorageMocks.Logger
}

func newTestMocks() testMocks {
	mocks := testMocks{
		conjurClient: conjurMocks.NewConjurClient(),
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
		m.conjurClient.CanExecute = false
	}
	if denyK8sRetrieve {
		m.kubeClient.CanRetrieve = false
	}
	if denyK8sUpdate {
		m.kubeClient.CanUpdate = false
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
		requiredSecrets,
		"someNamespace")
}

type assertFunc func(*testing.T, testMocks, error, string)
type expectedK8sSecrets map[string]map[string]string
type expectedMissingValues map[string][]string

func assertErrorContains(expErrStr string) assertFunc {
	return func(t *testing.T, _ testMocks,
		err error, desc string) {

		assert.Error(t, err, desc)
		assert.Contains(t, err.Error(), expErrStr, desc)
	}
}

func assertSecretsUpdated(expK8sSecrets expectedK8sSecrets,
	expMissingValues expectedMissingValues) assertFunc {
	return func(t *testing.T, mocks testMocks, err error, desc string) {
		assert.NoError(t, err, desc)
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
		for k8sSecretName, expMissingValue := range expMissingValues {
			actualSecretData := mocks.kubeClient.InspectSecret(k8sSecretName)
			for _, value := range actualSecretData {
				actualValue := string(value)
				newDesc := desc + ", Leaked secret value: " + actualValue
				assert.NotEqual(t, expMissingValue, actualValue, newDesc)
			}
		}
	}
}

func assertErrorLogged(msg string, args ...interface{}) assertFunc {
	return func(t *testing.T, mocks testMocks, err error, desc string) {
		errStr := fmt.Sprintf(msg, args...)
		newDesc := desc + ", error logged: " + errStr
		assert.True(t, mocks.logger.ErrorWasLogged(errStr), newDesc)
	}
}

func TestProvide(t *testing.T) {
	testCases := []struct {
		desc               string
		k8sSecrets         k8sStorageMocks.K8sSecrets
		requiredSecrets    []string
		denyConjurRetrieve bool
		denyK8sRetrieve    bool
		denyK8sUpdate      bool
		asserts            []assertFunc
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
				),
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
				assertErrorContains(messages.CSPFK021E),
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
				assertErrorContains(messages.CSPFK021E),
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
				assertErrorContains(fmt.Sprintf(messages.CSPFK034E, "custom error")),
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
				assertErrorContains(messages.CSPFK023E),
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
				assertErrorContains(messages.CSPFK021E),
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
				assertErrorContains(messages.CSPFK021E),
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
				assertErrorContains(messages.CSPFK021E),
			},
		},
	}

	for _, tc := range testCases {
		// Set up test case
		mocks := newTestMocks()
		mocks.setPermissions(tc.denyConjurRetrieve, tc.denyK8sRetrieve,
			tc.denyK8sUpdate)
		for secretName, secretData := range tc.k8sSecrets {
			mocks.kubeClient.AddSecret(secretName, secretData)
		}
		provider := mocks.newProvider(tc.requiredSecrets)

		// Run test case
		err := provider.Provide()

		// Confirm results
		for _, assert := range tc.asserts {
			assert(t, mocks, err, tc.desc)
		}
	}
}
