package pushtofile

import (
	"testing"

	conjurMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur/mocks"
	"github.com/stretchr/testify/assert"
)

type retrieveSecretsTestCase struct {
	description string
	secretSpecs map[string][]SecretSpec
	assert      func(t *testing.T, result map[string][]*secret, err error)
}

func (tc retrieveSecretsTestCase) Run(t *testing.T, mockSecretFetch MockSecretFetch) {
	t.Run(tc.description, func(t *testing.T) {
		s := createSecretGroups(tc.secretSpecs)
		ret, err := fetchSecretsForGroups(mockSecretFetch, &s)
		tc.assert(t, ret, err)
	})
}

func createSecretGroups(groupSpecs map[string][]SecretSpec) SecretGroups {
	secretGroups := SecretGroups{}
	for label, secretSpecs := range groupSpecs {
		secretGroup := SecretGroup{
			Label:       label,
			SecretSpecs: secretSpecs,
		}
		secretGroups = append(secretGroups, secretGroup)
	}
	return secretGroups
}

func findGroupValues(group map[string][]*secret, label string) []*secret {
	for key, secretGroup := range group {
		if key == label {
			return secretGroup
		}
	}
	return nil
}

func assertGoodResults(expectedGroupValues map[string][]*secret) func(*testing.T, map[string][]*secret, error) {
	return func(t *testing.T, result map[string][]*secret, err error) {

		if !assert.NoError(t, err) {
			return
		}
		for groupLabel, expValues := range expectedGroupValues {
			actualValues := findGroupValues(result, groupLabel)
			assert.NotNil(t, actualValues)
			assert.True(t, assert.EqualValues(t, actualValues, expValues))
		}
	}
}

var retrieveSecretsTestCases = []retrieveSecretsTestCase{
	{
		description: "Happy Case",
		secretSpecs: map[string][]SecretSpec{
			"cache": []SecretSpec{
				{Alias: "api-url", Id: "dev/openshift/api-url"},
				{Alias: "username", Id: "dev/openshift/username"},
				{Alias: "password", Id: "dev/openshift/password"},
			},
			"db": []SecretSpec{
				{Alias: "api-url", Id: "ci/openshift/api-url"},
				{Alias: "username", Id: "ci/openshift/username"},
				{Alias: "password", Id: "ci/openshift/password"},
			},
		},
		assert: assertGoodResults(map[string][]*secret{
			"cache": []*secret{
				{Alias: "api-url", Value: "https://postgres.example.com"},
				{Alias: "username", Value: "admin"},
				{Alias: "password", Value: "open-$e$ame"},
			},
			"db": []*secret{
				{Alias: "api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "username", Value: "administrator"},
				{Alias: "password", Value: "open-$e$ame"},
			},
		}),
	},
	{
		description: "Bad ID",
		secretSpecs: map[string][]SecretSpec{
			"cache": []SecretSpec{
				{Alias: "api-url", Id: "foo/openshift/bar"},
				{Alias: "username", Id: "dev/openshift/username"},
				{Alias: "password", Id: "dev/openshift/password"},
			},
			"db": []SecretSpec{
				{Alias: "api-url", Id: "ci/openshift/api-url"},
				{Alias: "username", Id: "ci/openshift/username"},
				{Alias: "password", Id: "ci/openshift/password"},
			},
		},
		assert: func(t *testing.T, result map[string][]*secret, err error) {
			assert.Contains(t, err.Error(), "Failed to retrieve secrets")
		},
	},
}

type MockSecretFetch struct {
	accessToken      conjurMocks.MockAccessToken
	conjurMockClient conjurMocks.ConjurMockClient
}

func (s MockSecretFetch) SecretFetcher(secretIds []string) (map[string][]byte, error) {

	accessTokenData, _ := s.accessToken.Read()
	return s.conjurMockClient.RetrieveSecrets(accessTokenData, secretIds)
}

func mockInit(s *MockSecretFetch) {
	s.conjurMockClient = conjurMocks.NewConjurMockClient()
	mockSecrets := map[string]string{
		"dev/openshift/api-url":  "https://postgres.example.com",
		"dev/openshift/username": "admin",
		"dev/openshift/password": "open-$e$ame",
		"ci/openshift/api-url":   "https://ci.postgres.example.com",
		"ci/openshift/username":  "administrator",
		"ci/openshift/password":  "open-$e$ame",
	}
	s.conjurMockClient.AddSecret(mockSecrets)
}

func TestRetrieveSecrets(t *testing.T) {
	var mockSecretFetch MockSecretFetch
	mockInit(&mockSecretFetch)
	for _, tc := range retrieveSecretsTestCases {
		tc.Run(t, mockSecretFetch)
	}
}
