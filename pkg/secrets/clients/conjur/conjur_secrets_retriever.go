package conjur

import (
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
)

// This interface is used to mock a ConjurSecretsRetriever struct
type ConjurSecretsRetrieverInterface interface {
	RetrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error)
}

// We create this empty struct so we have an object with the functions below
type ConjurSecretsRetriever struct{}

func (conjurSecretsRetriever ConjurSecretsRetriever) RetrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
	log.Info(messages.CSPFK003I, variableIDs)

	conjurClient, err := NewConjurClient(accessToken)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK033E)
	}

	retrievedSecrets, err := conjurClient.RetrieveBatchSecrets(variableIDs)
	if err != nil {
		return nil, err
	}

	return retrievedSecrets, nil
}
