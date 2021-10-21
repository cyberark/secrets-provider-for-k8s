package pushtofile

import (
	"fmt"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
)

// Secret describes how Conjur secrets are represented in the Push-to-File context.
type Secret struct {
	Alias string
	Value string
}

// FetchSecretsForGroups fetches the secrets for all the groups and returns
// map of [group name] to [a slice of secrets for the group]. Callers of this
// function should decorate any errors with messages.CSPFK052E
func FetchSecretsForGroups(
	depRetrieveSecrets conjur.RetrieveSecretsFunc,
	secretGroups []*SecretGroup,
) (map[string][]*Secret, error) {
	var err error
	secretsByGroup := map[string][]*Secret{}

	secretPaths := getAllPaths(secretGroups)
	secretValueById, err := depRetrieveSecrets(secretPaths)
	if err != nil {
		return nil, err
	}

	for _, group := range secretGroups {
		for _, spec := range group.SecretSpecs {
			sValue, ok := secretValueById[spec.Path]
			if !ok {
				err = fmt.Errorf(
					"secret with alias %q not present in fetched secrets",
					spec.Alias,
				)
				return nil, err
			}

			secretsByGroup[group.Name] = append(
				secretsByGroup[group.Name],
				&Secret{
					Alias: spec.Alias,
					Value: string(sValue),
				},
			)
		}
	}

	return secretsByGroup, err
}

func getAllPaths(secretGroups []*SecretGroup) []string {
	var ids []string
	for _, group := range secretGroups {
		for _, spec := range group.SecretSpecs {
			ids = append(ids, spec.Path)
		}
	}
	return ids
}
