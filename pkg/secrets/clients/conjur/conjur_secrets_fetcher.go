package conjur

import (
	"fmt"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	authnConfigProvider "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

type FetchSecretsFunc func(variableIDs []string) (map[string][]byte, error)

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

func fetchConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
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

	// TODO: maybe check retrievedSecrets to ensure it has all the secrets otherwise
	//  return an error

	return retrievedSecrets, nil
}

func NewConjurSecretFetcher(authnConfig authnConfigProvider.Config) (*conjurSecretFetcher, error)  {
	accessToken, err := memory.NewAccessToken()
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK001E)
	}

	authn, err := authenticator.NewWithAccessToken(authnConfig, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK009E)
	}

	return &conjurSecretFetcher{
		authn: authn,
	}, nil
}

type conjurSecretFetcher struct {
	authn *authenticator.Authenticator
}

func (fetcher conjurSecretFetcher) Fetch(secretIds []string) (map[string][]byte, error) {
	authn := fetcher.authn

	// Only get an access token when it is needed
	err := authn.Authenticate()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK010E)
	}

	// NOTE: the token is cleaned up by whoever created it!
	accessTokenData, err := authn.AccessToken.Read()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK002E)
	}
	// Always delete access token. The deletion idempotent and never fails
	defer authn.AccessToken.Delete()

	return fetchConjurSecrets(accessTokenData, secretIds)
}
