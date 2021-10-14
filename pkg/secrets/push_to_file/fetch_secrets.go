package push_to_file

import "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"

func FetchSecretsForGroups(
	fetchSecrets conjur.FetchSecretsFunc,
	secretGroups []*SecretGroup,
) (
	map[string][]*Secret, // map[group name] => group secret vales
	error,
) {
	// Gather secret paths
	var secretPaths []string
	var uniqueSecretPaths = map[string]struct{}{}
	for _, group := range secretGroups {
		specs := group.ResolvedSecretSpecs()
		for _, spec := range specs {
			if _, ok := uniqueSecretPaths[spec.Path]; ok {
				continue
			}

			uniqueSecretPaths[spec.Path] = struct{}{}
			secretPaths = append(secretPaths, spec.Path)
		}
	}

	secretValueByPath, err := fetchSecrets(secretPaths)
	if err != nil {
		return nil, err
	}

	// Gather secret values
	var secretsByGroup = map[string][]*Secret{}
	for _, group := range secretGroups {
		specs := group.ResolvedSecretSpecs()

		groupSecrets := make([]*Secret, len(specs))

		for i, spec := range specs {
			secretValue := secretValueByPath[spec.Path]
			groupSecrets[i] = &Secret{
				Alias: spec.Alias,
				Value: string(secretValue),
			}
		}
		secretsByGroup[group.Name] = groupSecrets
	}

	return secretsByGroup, nil
}
