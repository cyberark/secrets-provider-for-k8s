package conjur

import (
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

type RetrieveConjurSecretsFunc func(accessToken []byte, variableIDs []string) (map[string][]byte, error)

func RetrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
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
