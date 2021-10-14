package pushtofile

import (
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
)

type secret struct {
	Alias string
	Value string
}
type fetcher interface {
	SecretFetcher(secretIds []string) (map[string][]byte, error)
}

type conjurSecretFetch struct {
	accessToken access_token.AccessToken
}

func (s conjurSecretFetch) SecretFetcher(secretIds []string) (map[string][]byte, error) {
	accessTokenData, err := s.accessToken.Read()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK002E)
	}
	return conjur.RetrieveConjurSecrets(accessTokenData, secretIds)
}

// FetchSecretsForGroups parses the SecretsGroup, gets
// the secrets from Conjur and updates the SecretsGroup with the secret
func FetchSecretsForGroups(secretGroups *SecretGroups,
	accessToken access_token.AccessToken) (map[string][]*secret, error) {
	var conjurSecretFetcher = conjurSecretFetch{accessToken}
	return fetchSecretsForGroups(conjurSecretFetcher, secretGroups)
}

func getAllIds(secretGroups *SecretGroups) []string {
	ids := []string{}
	for _, group := range *secretGroups {
		for _, spec := range group.SecretSpecs {
			ids = append(ids, spec.Id)
		}
	}
	return ids
}

func fetchSecretsForGroups(secretsFetcherFunc fetcher,
	secretGroups *SecretGroups,
) (map[string][]*secret, error) {

	secretsValues := map[string][]*secret{}

	ids := getAllIds(secretGroups)
	retrieved, err := secretsFetcherFunc.SecretFetcher(ids)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK052E, err.Error())
	}
	for _, group := range *secretGroups {
		for _, spec := range group.SecretSpecs {
			for id, retSecret := range retrieved {
				if strings.Contains(id, spec.Id) {
					fetchedSecret := new(secret)
					fetchedSecret.Alias = spec.Alias
					fetchedSecret.Value = string(retSecret)
					secretsValues[group.Label] = append(secretsValues[group.Label], fetchedSecret)
				}
			}
		}
	}
	return secretsValues, err
}
