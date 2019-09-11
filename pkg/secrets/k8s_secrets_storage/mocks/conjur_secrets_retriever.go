package mocks

import (
	"errors"
	"fmt"
)

type MockConjurSecretsRetriever struct {
	Permissions map[string]bool
}

// Reads Conjur secrets from the mock DB and returns a map from variable IDs to the corresponding secrets.
func (ConjurSecretsRetriever MockConjurSecretsRetriever) RetrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
	conjurSecrets := make(map[string][]byte)

	if !ConjurSecretsRetriever.Permissions["execute"] {
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
