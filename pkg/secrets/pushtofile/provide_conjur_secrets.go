package pushtofile

import (
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
)

type fileProvider struct {
	retrieveSecretsFunc conjur.RetrieveSecretsFunc
	secretGroups        []*SecretGroup
}

// NewProvider creates a new provider for Push-to-File mode.
func NewProvider(retrieveSecretsFunc conjur.RetrieveSecretsFunc, secretsBasePath string, annotations map[string]string) (*fileProvider, []error) {
	secretGroups, err := NewSecretGroups(secretsBasePath, annotations)
	if err != nil {
		return nil, err
	}

	return &fileProvider{
		retrieveSecretsFunc: retrieveSecretsFunc,
		secretGroups:        secretGroups,
	}, nil
}

// Provide implements a ProviderFunc to retrieve and push secrets to the filesystem.
func (p fileProvider) Provide() error {
	return provideWithDeps(
		p.secretGroups,
		p.retrieveSecretsFunc,
		openFileAsWriteCloser,
		pushToWriter,
	)
}

func provideWithDeps(
	groups []*SecretGroup,
	retrieveSecretsFunc conjur.RetrieveSecretsFunc,
	depOpenWriteCloser openWriteCloserFunc,
	depPushToWriter pushToWriterFunc,
) error {
	secretsByGroup, err := FetchSecretsForGroups(retrieveSecretsFunc, groups)
	if err != nil {
		return err
	}

	for _, group := range groups {
		err := group.pushToFileWithDeps(
			depOpenWriteCloser,
			depPushToWriter,
			secretsByGroup[group.Name],
		)
		if err != nil {
			return err
		}
	}

	log.Info(messages.CSPFK015I)
	return nil
}
