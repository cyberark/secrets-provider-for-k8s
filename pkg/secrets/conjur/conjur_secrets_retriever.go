package conjur

import (
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
)

/*
	This struct holds a ConjurClient and retrieves Conjur secrets with it.

 	For example:

	// the c-tor needs a Conjur access token to initialize the client
	conjurSecretsRetriever, err := conjur.NewConjurSecretsRetriever(accessToken)
	if err != nil {
		return nil, log.RecorderError(log.CSPFK069E)
	}

	// this method receives a list of Conjur variables to retrieve
	retrievedConjurSecrets, err := conjurSecretsRetriever.RetrieveConjurSecrets(variableIDs)
	if err != nil {
		return log.RecorderError(log.CSPFK026E)
	}
*/
type ConjurSecretsRetriever struct {
	conjurClient ConjurClient
}

func NewConjurSecretsRetriever(accessToken []byte) (*ConjurSecretsRetriever, error) {
	conjurClient, err := NewConjurClient(accessToken)
	if err != nil {
		return nil, log.RecorderError(log.CSPFK020E)
	}

	return &ConjurSecretsRetriever{
		conjurClient: conjurClient,
	}, nil
}

func (conjurSecretsRetriever ConjurSecretsRetriever) RetrieveConjurSecrets(variableIDs []string) (map[string][]byte, error) {
	log.InfoLogger.Println(log.CSPFK018I, variableIDs)

	retrievedSecrets, err := conjurSecretsRetriever.conjurClient.RetrieveBatchSecrets(variableIDs)
	if err != nil {
		return nil, log.RecorderError(log.CSPFK021E, err.Error())
	}

	return retrievedSecrets, nil
}
