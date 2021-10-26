package pushtofile

import (
	"fmt"
)

type secret struct {
	Alias string
	Value string
}

type FetchSecretsFunc func(secretPaths []string) (map[string][]byte, error)

// FetchSecretsForGroups fetches the secrets for all the groups and returns
// map of [group name] to [a slice of secrets for the group]. Callers of this
// function should decorate any errors with messages.CSPFK052E
func FetchSecretsForGroups(
	depFetchSecrets FetchSecretsFunc,
	secretGroups []*SecretGroup,
) (map[string][]*secret, error) {
	var err error
	secretsByGroup := map[string][]*secret{}

	secretPaths := getAllPaths(secretGroups)
	secretValueById, err := depFetchSecrets(secretPaths)
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
				&secret{
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
