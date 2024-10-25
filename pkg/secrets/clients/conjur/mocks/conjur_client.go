package mocks

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

type ConjurMockClient struct {
	ReturnNoSecrets     bool
	ErrOnExecute        error
	Database            map[string]string
	AutoGenerateResults bool
}

func (mc *ConjurMockClient) RetrieveSecrets(variableIDs []string, _ context.Context) (map[string][]byte, error) {
	return mc.RetrieveBatchSecretsSafe(variableIDs)
}

func (mc *ConjurMockClient) RetrieveBatchSecretsSafe(variableIDs []string) (map[string][]byte, error) {
	if mc.ErrOnExecute != nil {
		return nil, mc.ErrOnExecute
	}

	if mc.ReturnNoSecrets {
		return map[string][]byte{}, nil
	}

	secrets := make(map[string][]byte)

	if mc.AutoGenerateResults {
		// Generate random values for all the secrets requested
		for _, id := range variableIDs {
			if id == "error" {
				return nil, errors.New("error")
			}
			fullID := fmt.Sprintf("conjur:variable:%s", id)
			secrets[fullID] = []byte("secret")
		}
	} else {
		// Return secrets from the Database map
		for _, secretID := range variableIDs {
			if secretID == "*" {
				secrets = mc.getAllSecrets()
				if len(secrets) == 0 {
					return nil, fmt.Errorf(messages.CSPFK034E, "no variables to retrieve")
				}
				return secrets, nil
			}

			// Check if the secret exists in the mock Conjur DB
			variableData, ok := mc.Database[secretID]
			if !ok {
				return nil, errors.New("no_conjur_secret_error")
			}
			secrets[secretID] = []byte(variableData)
		}
	}

	return secrets, nil
}

func (mc *ConjurMockClient) Resources(filter *conjurapi.ResourceFilter) (resources []map[string]interface{}, err error) {
	if mc.ReturnNoSecrets {
		return []map[string]interface{}{}, nil
	}

	if !mc.AutoGenerateResults {
		// We have no need for this test case at this point
		return nil, fmt.Errorf("Not implemented")
	}

	// Generate random secret results, enough to test pagination.
	// This code is based on the mock used in External Secrets Operator, see
	// https://github.com/external-secrets/external-secrets/blob/02c6f625bd24c411f34ca1e390fdf68fc2da960c/pkg/provider/conjur/fake/fake.go
	policyID := "conjur:policy:root"
	if filter.Offset == 0 {
		// First "page" of secrets: 2 static ones and 98 random ones
		secrets := []map[string]interface{}{
			{
				"id": "conjur:variable:secret1",
				"annotations": []interface{}{
					map[string]interface{}{
						"name":  "conjur/kind",
						"value": "dummy",
					},
				},
			},
			{
				"id":    "conjur:variable:secret2",
				"owner": "conjur:policy:admin1",
				"annotations": []interface{}{
					map[string]interface{}{
						"name":   "Description",
						"policy": policyID,
						"value":  "Lorem ipsum dolor sit amet",
					},
					map[string]interface{}{
						"name":   "conjur/kind",
						"policy": policyID,
						"value":  "password",
					},
				},
				"permissions": map[string]string{
					"policy":    policyID,
					"privilege": "update",
					"role":      "conjur:group:admins",
				},
				"policy": policyID,
			},
		}
		// Add 98 random secrets so we can simulate a full "page" of 100 secrets
		secrets = append(secrets, generateRandomSecrets(98)...)
		return secrets, nil
	} else if filter.Offset == 100 {
		// Second "page" of secrets: 100 random ones
		return generateRandomSecrets(100), nil
	}

	// Add 50 random secrets so we can simulate a partial "page" of 50 secrets, for a total of 250 secrets
	return generateRandomSecrets(50), nil
}

func NewConjurMockClient() *ConjurMockClient {
	database := map[string]string{
		"conjur_variable1":             "conjur_secret1",
		"conjur_variable2":             "conjur_secret2",
		"conjur_variable_empty_secret": "",
	}

	return &ConjurMockClient{
		ErrOnExecute: nil,
		Database:     database,
	}
}

func (mc *ConjurMockClient) AddSecrets(
	secrets map[string]string,
) {
	for id, secret := range secrets {
		mc.Database[id] = secret
	}
}

func (mc *ConjurMockClient) ClearSecrets() {
	mc.Database = make(map[string]string)
}

func generateRandomSecrets(count int) []map[string]interface{} {
	var secrets []map[string]interface{}
	for i := 0; i < count; i++ {
		//nolint:gosec
		randomNumber := rand.Intn(10000000)
		secrets = append(secrets, generateRandomSecret(randomNumber))
	}
	return secrets
}

func generateRandomSecret(num int) map[string]interface{} {
	return map[string]interface{}{
		"id": fmt.Sprintf("conjur:variable:random/var_%d", num),
		"annotations": []map[string]interface{}{
			{
				"name":  "random_number",
				"value": fmt.Sprintf("%d", num),
			},
		},
		"policy": "conjur:policy:random",
	}
}

func (mc *ConjurMockClient) getAllSecrets() map[string][]byte {
	res := make(map[string][]byte)

	for id, secret := range mc.Database {
		res[id] = []byte(secret)
	}

	return res
}
