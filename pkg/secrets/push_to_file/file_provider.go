package push_to_file

import (
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
)

// fileProvider represents the provider for pushing secrets to Kubernetes
type fileProvider struct {
	secretGroups []*SecretGroup
}

func NewProvider(annotations map[string]string) (*fileProvider, error)  {
	secretGroups, err := NewSecretGroups(annotations)
	if err != nil {
		return nil, err
	}

	return &fileProvider{secretGroups: secretGroups}, nil
}

// This method is implemented for implementing the Provider interface.
func (p fileProvider) Provide(fetchSecrets conjur.FetchSecretsFunc) error {
	secretGroups := p.secretGroups

	log.Info("Fetching for secret groups")
	for _, g := range secretGroups {
		log.Debug("%#v", g)
	}
	secretsByGroup, err := FetchSecretsForGroups(
		fetchSecrets,
		secretGroups,
	)
	if err != nil {
		return err
	}

	// Write secrets to file
	for _, group := range secretGroups {
		log.Info("Processing template for group %q to file path %q\n", group.Name, group.FilePath)

		err := group.PushToFile(secretsByGroup[group.Name])
		// TODO: accumulate errors instead of sending only one. This probably needs the
		// Provider interface to change
		if err != nil {
			return err
		}
	}

	return nil
}
