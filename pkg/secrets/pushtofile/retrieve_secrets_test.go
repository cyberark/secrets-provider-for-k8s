package pushtofile

import (
	"context"
	"testing"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	conjurMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur/mocks"
	"github.com/stretchr/testify/assert"
)

type retrieveSecretsTestCase struct {
	description string
	secretSpecs map[string][]SecretSpec
	assert      func(t *testing.T, result map[string][]*Secret, err error)
}

func (tc retrieveSecretsTestCase) Run(
	t *testing.T,
	depFetchSecrets conjur.RetrieveSecretsFunc,
) {
	t.Run(tc.description, func(t *testing.T) {
		s := createSecretGroups(tc.secretSpecs)
		ret, err := FetchSecretsForGroups(depFetchSecrets, s, context.Background())

		tc.assert(t, ret, err)
	})
}

func createSecretGroups(groupSpecs map[string][]SecretSpec) []*SecretGroup {
	var secretGroups []*SecretGroup
	for name, secretSpecs := range groupSpecs {
		secretGroup := &SecretGroup{
			Name:        name,
			SecretSpecs: secretSpecs,
		}
		secretGroups = append(secretGroups, secretGroup)
	}

	return secretGroups
}

func findGroupValues(group map[string][]*Secret, label string) []*Secret {
	for key, secretGroup := range group {
		if key == label {
			return secretGroup
		}
	}

	return nil
}

func assertGoodResults(expectedGroupValues map[string][]*Secret) func(*testing.T, map[string][]*Secret, error) {
	return func(t *testing.T, result map[string][]*Secret, err error) {
		if !assert.NoError(t, err) {
			return
		}

		for groupLabel, expValues := range expectedGroupValues {
			actualValues := findGroupValues(result, groupLabel)
			assert.NotNil(t, actualValues)
			// We use EqualValues instead of ElementsMatch because we want to
			// test the ordering, particularly for the Fetch All case.
			assert.EqualValues(t, expValues, actualValues)
		}
	}
}

var retrieveSecretsTestCases = []retrieveSecretsTestCase{
	{
		description: "Happy Case",
		secretSpecs: map[string][]SecretSpec{
			"cache": {
				{Alias: "api-url", Path: "dev/openshift/api-url"},
				{Alias: "username", Path: "dev/openshift/username"},
				{Alias: "password", Path: "dev/openshift/password"},
			},
			"db": {
				{Alias: "api-url", Path: "ci/openshift/api-url"},
				{Alias: "username", Path: "ci/openshift/username"},
				{Alias: "password", Path: "ci/openshift/password"},
			},
		},
		assert: assertGoodResults(map[string][]*Secret{
			"cache": {
				{Alias: "api-url", Value: "https://postgres.example.com"},
				{Alias: "username", Value: "admin"},
				{Alias: "password", Value: "open-$e$ame"},
			},
			"db": {
				{Alias: "api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "username", Value: "administrator"},
				{Alias: "password", Value: "open-$e$ame"},
			},
		}),
	},
	{
		description: "Happy Base64 Case",
		secretSpecs: map[string][]SecretSpec{
			"cache": {
				{Alias: "api-url", Path: "dev/openshift/api-url"},
				{Alias: "username", Path: "dev/openshift/username"},
				{Alias: "password", Path: "dev/openshift/password",
					ContentType: "text"},
			},
			"db": {
				{Alias: "api-url", Path: "ci/openshift/api-url"},
				{Alias: "username", Path: "ci/openshift/username"},
				{Alias: "encoded-password", Path: "ci/openshift/encoded-password",
					ContentType: "base64"},
			},
		},
		assert: assertGoodResults(map[string][]*Secret{
			"cache": {
				{Alias: "api-url", Value: "https://postgres.example.com"},
				{Alias: "username", Value: "admin"},
				{Alias: "password", Value: "open-$e$ame"},
			},
			"db": {
				{Alias: "api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "username", Value: "administrator"},
				{Alias: "encoded-password", Value: "open-$e$ame"},
			},
		}),
	},
	{
		description: "Cannot decode Base64 Case",
		secretSpecs: map[string][]SecretSpec{
			"db": {
				{Alias: "api-url", Path: "ci/openshift/api-url"},
				{Alias: "username", Path: "ci/openshift/username"},
				{Alias: "encoded-password", Path: "ci/openshift/password",
					ContentType: "base64"},
			},
		},
		assert: assertGoodResults(map[string][]*Secret{
			"db": {
				{Alias: "api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "username", Value: "administrator"},
				{Alias: "encoded-password", Value: "open-$e$ame"},
			},
		}),
	},
	{
		description: "Bad ID",
		secretSpecs: map[string][]SecretSpec{
			"cache": {
				{Alias: "api-url", Path: "foo/openshift/bar"},
				{Alias: "username", Path: "dev/openshift/username"},
				{Alias: "password", Path: "dev/openshift/password"},
			},
			"db": {
				{Alias: "api-url", Path: "ci/openshift/api-url"},
				{Alias: "username", Path: "ci/openshift/username"},
				{Alias: "password", Path: "ci/openshift/password"},
			},
		},
		assert: func(t *testing.T, result map[string][]*Secret, err error) {
			assert.Contains(t, err.Error(), "no_conjur_secret_error")
		},
	},
	{
		description: "Fetch All Happy Case",
		secretSpecs: map[string][]SecretSpec{
			"unified": {
				{Path: "*"},
			},
		},
		assert: assertGoodResults(map[string][]*Secret{
			// Expect all secrets to be fetched, with full paths as aliases
			"unified": {
				{Alias: "ci/openshift/api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "ci/openshift/encoded-password", Value: "b3Blbi0kZSRhbWU="},
				{Alias: "ci/openshift/password", Value: "open-$e$ame"},
				{Alias: "ci/openshift/username", Value: "administrator"},
				{Alias: "conjur_variable1", Value: "conjur_secret1"},
				{Alias: "conjur_variable2", Value: "conjur_secret2"},
				{Alias: "conjur_variable_empty_secret", Value: ""},
				{Alias: "dev/openshift/api-url", Value: "https://postgres.example.com"},
				{Alias: "dev/openshift/password", Value: "open-$e$ame"},
				{Alias: "dev/openshift/username", Value: "admin"},
			},
		}),
	},
	{
		description: "Fetch All Base64",
		secretSpecs: map[string][]SecretSpec{
			"unified": {
				{Path: "*", ContentType: "base64"},
			},
		},
		assert: assertGoodResults(map[string][]*Secret{
			// Expect all secrets to be fetched, with full paths as aliases
			"unified": {
				{Alias: "ci/openshift/api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "ci/openshift/encoded-password", Value: "open-$e$ame"},
				{Alias: "ci/openshift/password", Value: "open-$e$ame"},
				{Alias: "ci/openshift/username", Value: "administrator"},
				{Alias: "conjur_variable1", Value: "conjur_secret1"},
				{Alias: "conjur_variable2", Value: "conjur_secret2"},
				{Alias: "conjur_variable_empty_secret", Value: ""},
				{Alias: "dev/openshift/api-url", Value: "https://postgres.example.com"},
				{Alias: "dev/openshift/password", Value: "open-$e$ame"},
				{Alias: "dev/openshift/username", Value: "admin"},
			},
		}),
	},
	{
		description: "Fetch All with Other Paths",
		secretSpecs: map[string][]SecretSpec{
			"all": {
				{Path: "*"},
			},
			"db": {
				{Alias: "api-url", Path: "ci/openshift/api-url"},
				{Alias: "username", Path: "ci/openshift/username"},
				{Alias: "password", Path: "ci/openshift/password"},
			},
		},
		assert: assertGoodResults(map[string][]*Secret{
			// Expect all secrets to be fetched, with full paths as aliases
			"all": {
				{Alias: "ci/openshift/api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "ci/openshift/encoded-password", Value: "b3Blbi0kZSRhbWU="},
				{Alias: "ci/openshift/password", Value: "open-$e$ame"},
				{Alias: "ci/openshift/username", Value: "administrator"},
				{Alias: "conjur_variable1", Value: "conjur_secret1"},
				{Alias: "conjur_variable2", Value: "conjur_secret2"},
				{Alias: "conjur_variable_empty_secret", Value: ""},
				{Alias: "dev/openshift/api-url", Value: "https://postgres.example.com"},
				{Alias: "dev/openshift/password", Value: "open-$e$ame"},
				{Alias: "dev/openshift/username", Value: "admin"},
			},
			"db": {
				{Alias: "api-url", Value: "https://ci.postgres.example.com"},
				{Alias: "username", Value: "administrator"},
				{Alias: "password", Value: "open-$e$ame"},
			},
		}),
	},
}

type mockSecretFetcher struct {
	conjurMockClient *conjurMocks.ConjurMockClient
}

func (s mockSecretFetcher) Fetch(secretPaths []string, ctx context.Context) (map[string][]byte, error) {
	return s.conjurMockClient.RetrieveSecrets(secretPaths, context.Background())
}

func newMockSecretFetcher() mockSecretFetcher {
	m := mockSecretFetcher{
		conjurMockClient: conjurMocks.NewConjurMockClient(),
	}

	m.conjurMockClient.AddSecrets(
		map[string]string{
			"dev/openshift/api-url":         "https://postgres.example.com",
			"dev/openshift/username":        "admin",
			"dev/openshift/password":        "open-$e$ame",
			"ci/openshift/api-url":          "https://ci.postgres.example.com",
			"ci/openshift/username":         "administrator",
			"ci/openshift/password":         "open-$e$ame",
			"ci/openshift/encoded-password": "b3Blbi0kZSRhbWU=",
		},
	)

	return m
}

func TestRetrieveSecrets(t *testing.T) {
	m := newMockSecretFetcher()

	for _, tc := range retrieveSecretsTestCases {
		tc.Run(t, m.Fetch)
	}
}

func TestGetAllPaths(t *testing.T) {
	// Define test cases
	testCases := []struct {
		description        string
		secretPathsByGroup map[string][]SecretSpec
		expectedPaths      []string
	}{
		{
			description: "Single secret group, no duplicated paths",
			secretPathsByGroup: map[string][]SecretSpec{
				"group-1": {
					{Alias: "var1", Path: "path/var1"},
					{Alias: "var2", Path: "path/var2"},
				},
			},
			expectedPaths: []string{"path/var1", "path/var2"},
		},
		{
			description: "Single secret group, duplicated path",
			secretPathsByGroup: map[string][]SecretSpec{
				"group-1": {
					{Alias: "var1", Path: "path/var1"},
					{Alias: "var2", Path: "path/var1"},
				},
			},
			expectedPaths: []string{"path/var1"},
		},
		{
			description: "Multiple secret groups, no duplicated path",
			secretPathsByGroup: map[string][]SecretSpec{
				"group-1": {
					{Alias: "var1", Path: "path/var1"},
					{Alias: "var2", Path: "path/var2"},
				},
				"group-2": {
					{Alias: "var3", Path: "path/var3"},
					{Alias: "var4", Path: "path/var4"},
				},
			},
			expectedPaths: []string{"path/var1", "path/var2", "path/var3", "path/var4"},
		},
		{
			description: "Multiple secret groups, duplicated path",
			secretPathsByGroup: map[string][]SecretSpec{
				"group-1": {
					{Alias: "var1", Path: "path/var1"},
					{Alias: "var2", Path: "path/var2"},
				},
				"group-2": {
					{Alias: "var3", Path: "path/var1"},
					{Alias: "var4", Path: "path/var4"},
				},
			},
			expectedPaths: []string{"path/var1", "path/var2", "path/var4"},
		},
		{
			description: "Fetch all secrets",
			secretPathsByGroup: map[string][]SecretSpec{
				"group-1": {
					{Path: "*"},
				},
			},
			expectedPaths: []string{"*"},
		},
		{
			description: "Fetch all secrets with other paths",
			secretPathsByGroup: map[string][]SecretSpec{
				"group-1": {
					{Alias: "var1", Path: "path/var1"},
					{Alias: "var2", Path: "path/var2"},
					{Path: "*"},
				},
				"group-2": {
					{Alias: "var3", Path: "path/var3"},
					{Alias: "var4", Path: "path/var4"},
				},
			},
			expectedPaths: []string{"*"},
		},
	}

	for _, tc := range testCases {
		// Set up a slice of SecretGroups to test
		secretGroups := []*SecretGroup{}
		for _, specs := range tc.secretPathsByGroup {
			secretGroup := SecretGroup{}
			secretGroup.SecretSpecs = append([]SecretSpec{}, specs...)
			secretGroups = append(secretGroups, &secretGroup)
		}

		// Run test case
		paths := getAllPaths(secretGroups)

		// Verify results
		assert.ElementsMatch(t, paths, tc.expectedPaths)
	}
}
