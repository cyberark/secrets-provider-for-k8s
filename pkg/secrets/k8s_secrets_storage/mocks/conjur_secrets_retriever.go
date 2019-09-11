package mocks

import (
	"errors"
	"fmt"
)

/*
	Determines if Conjur secrets have 'execute' privileges by a representative boolean value. We don't dive deeper
	into the granularity at the level of each Conjur variable because for this suite of tests we aren't concerned with
	if some Conjur secrets have permissions and others don't. Our main priority is validating that those
	secrets with 'execute' permissions can be fetched.
*/
var ExecutePermission bool

type MockConjurSecretsRetriever struct{}

// Reads Conjur secrets from the mock DB and returns a map from variable IDs to the corresponding secrets.
func (ConjurSecretsRetriever MockConjurSecretsRetriever) RetrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
	conjurSecrets := make(map[string][]byte)

	if !ExecutePermission {
		return nil, errors.New("custom error")
	}

	for _, variableId := range variableIDs {
		// Check if the secret exists in the mock Conjur DB
		if _, ok := MockConjurDB[variableId]; !ok {
			return nil, errors.New("no_conjur_secret_error")
		}

		fullVariableId := fmt.Sprintf("account:variable:%s", variableId)
		conjurSecrets[fullVariableId] = MockConjurDB[variableId]
	}

	return conjurSecrets, nil
}
