package mocks

import "fmt"

type MockConjurSecretsRetriever struct{}

// Reads Conjur secrets from the mock DB and returns a map from variable IDs to the corresponding secrets.
func (ConjurSecretsRetriever MockConjurSecretsRetriever) RetrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
	conjurSecrets := make(map[string][]byte)
	for _, variableId := range variableIDs {
		fullVariableId := fmt.Sprintf("account:variable:%s", variableId)
		conjurSecrets[fullVariableId] = MockConjurDB[variableId]
	}

	return conjurSecrets, nil
}
