package conjur

import (
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

type FetchSecretsFunc func(variableIDs []string) (map[string][]byte, error)

func fetchConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
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

func NewConjurSecretFetcher(authn *authenticator.Authenticator) conjurSecretFetcher  {
	return conjurSecretFetcher{
		authn: authn,
	}
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

	return fetchConjurSecrets(accessTokenData, secretIds)
}
