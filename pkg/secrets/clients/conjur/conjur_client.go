package conjur

import (
	"github.com/cyberark/conjur-api-go/conjurapi"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
)

/*
	Client for communication with Conjur. In this project it is used only for
    batch secrets retrieval so we expose only this method of the client.

	The name ConjurClient also improves readability as Client can be ambiguous.
*/
type ConjurClient interface {
	RetrieveBatchSecrets([]string) (map[string][]byte, error)
}

func NewConjurClient(tokenData []byte) (ConjurClient, error) {
	log.InfoLogger.Printf(messages.CSPFK103I)
	config, err := conjurapi.LoadConfig()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK031E, err.Error())
	}

	client, err := conjurapi.NewClientFromToken(config, string(tokenData))
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK032E, err.Error())
	}

	return client, nil
}
