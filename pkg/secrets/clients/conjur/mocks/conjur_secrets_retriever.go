package mocks

import (
	"errors"
	"fmt"
)

/*
	Determines if Conjur secrets have 'execute' privileges by mapping `execute` to true or false. We don't
	dive deeper into the granularity at the level of each Conjur variable because for this suite of tests we
	aren't concerned with if some Conjur secrets have permissions and others don't. Our main priority is
	validating that those secrets with 'execute' permissions can be fetched.
*/

type ConjurMockClient struct {
	CanExecute bool
	// TODO: CanExecute is really just used to assert on the presence of errors
	// 	and should probably just be an optional error.
	Database   map[string]string
}

func (c ConjurMockClient) RetrieveSecrets (accessToken []byte, variableIDs []string) (map[string][]byte, error) {
	conjurSecrets := make(map[string][]byte)

	if !c.CanExecute {
		return nil, errors.New("custom error")
	}

	for _, variableId := range variableIDs {
		// Check if the secret exists in the mock Conjur DB
		variableData, ok := c.Database[variableId]
		if !ok {
			return nil, errors.New("no_conjur_secret_error")
		}

		fullVariableId := fmt.Sprintf("account:variable:%s", variableId)
		conjurSecrets[fullVariableId] = []byte(variableData)
	}

	return conjurSecrets, nil
}

func NewConjurMockClient() ConjurMockClient {
	database := map[string]string{
		"conjur_variable1": "conjur_secret1",
		"conjur_variable2": "conjur_secret2",
		"conjur_variable_empty_secret": "",
	}

	return ConjurMockClient{
		CanExecute: true,
		Database:   database,
	}
}

