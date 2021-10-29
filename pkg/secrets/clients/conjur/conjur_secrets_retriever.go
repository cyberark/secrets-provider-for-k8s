package conjur

import (
	"fmt"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

type conjurSecretRetriever struct {
	authn *authenticator.Authenticator
}

// RetrieveSecretsFunc defines a function type for retrieving secrets.
type RetrieveSecretsFunc func(variableIDs []string) (map[string][]byte, error)

// NewConjurSecretRetriever creates a new conjurSecretRetriever and Authenticator
// given an authenticator config.
func NewConjurSecretRetriever(authnConfig config.Config) (*conjurSecretRetriever, error) {
	accessToken, err := memory.NewAccessToken()
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK001E)
	}

	authn, err := authenticator.NewWithAccessToken(authnConfig, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK009E)
	}

	return &conjurSecretRetriever{
		authn: authn,
	}, nil
}

// Retrieve implements a RetrieveSecretsFunc for a given conjurSecretRetriever.
// Authenticates the client, and retrieves a given batch of variables from Conjur.
func (retriever conjurSecretRetriever) Retrieve(variableIDs []string) (map[string][]byte, error) {
	authn := retriever.authn

	err := authn.Authenticate()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK010E)
	}

	accessTokenData, err := authn.AccessToken.Read()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK002E)
	}
	// Always delete the access token. The deletion is idempotent and never fails
	defer authn.AccessToken.Delete()

	return retrieveConjurSecrets(accessTokenData, variableIDs)
}

func retrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
	log.Info(messages.CSPFK003I, variableIDs)

	conjurClient, err := NewConjurClient(accessToken)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK033E)
	}

	retrievedSecretsByFullIDs, err := conjurClient.RetrieveBatchSecrets(variableIDs)
	if err != nil {
		return nil, err
	}

	// Normalise secret IDs from batch secrets back to <variable_id>
	var retrievedSecrets = map[string][]byte{}
	for id, secret := range retrievedSecretsByFullIDs {
		retrievedSecrets[normaliseVariableId(id)] = secret
		delete(retrievedSecretsByFullIDs, id)
	}

	return retrievedSecrets, nil
}

// The variable ID can be in the format "<account>:variable:<variable_id>". This function
// just makes sure that if a variable is of the form "<account>:variable:<variable_id>"
// we normalise it to "<variable_id>", otherwise we just leave it be!
func normaliseVariableId(fullVariableId string) string {
	variableIdParts := strings.SplitN(fullVariableId, ":", 3)
	if len(variableIdParts) == 3 {
		return variableIdParts[2]
	}

	return fullVariableId
}
