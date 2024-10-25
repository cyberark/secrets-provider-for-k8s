package conjur

import (
	"testing"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur/mocks"
	"github.com/stretchr/testify/assert"
)

func TestRetrieveConjurSecrets(t *testing.T) {
	testCases := []struct {
		name            string
		variableIDs     []string
		expectedSecrets map[string][]byte
		expectError     bool
	}{
		{
			name: "Retrieve secrets successfully",
			variableIDs: []string{
				"secret1",
				"secret2",
				"secret3",
			},
			expectedSecrets: map[string][]byte{
				"secret1": []byte("secret"),
				"secret2": []byte("secret"),
				"secret3": []byte("secret"),
			},
		},
		{
			name:            "Return no secrets",
			variableIDs:     []string{},
			expectedSecrets: nil,
			expectError:     true,
		},
		{
			name:            "Return error",
			variableIDs:     []string{"error"},
			expectedSecrets: nil,
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &mocks.ConjurMockClient{
				AutoGenerateResults: true,
			}
			secrets, err := retrieveConjurSecrets(client, tc.variableIDs)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedSecrets, secrets)
		})
	}
}

func TestRetrieveConjurSecretsAll(t *testing.T) {
	testCases := []struct {
		name                 string
		maxSecretsCount      int
		expectedSecretsCount int
		expectError          bool
	}{
		{
			name:                 "Retrieve all secrets successfully",
			maxSecretsCount:      500,
			expectedSecretsCount: 250,
		},
		{
			name:                 "Return 100 secrets",
			maxSecretsCount:      100,
			expectedSecretsCount: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetchAllMaxSecrets = tc.maxSecretsCount
			client := &mocks.ConjurMockClient{
				AutoGenerateResults: true,
			}
			secrets, err := retrieveConjurSecretsAll(client)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Len(t, secrets, tc.expectedSecretsCount)
		})

		t.Cleanup(func() {
			// Set back to the default value
			fetchAllMaxSecrets = 500
		})
	}
}

func TestRetrieveConjurSecretsAllWithNoSecrets(t *testing.T) {
	client := &mocks.ConjurMockClient{
		ReturnNoSecrets: true,
	}
	secrets, err := retrieveConjurSecretsAll(client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CSPFK034E")
	assert.Contains(t, err.Error(), "no variables to retrieve")
	assert.Len(t, secrets, 0)
}
