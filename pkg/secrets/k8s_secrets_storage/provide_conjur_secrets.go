package k8s_secrets_storage

import (
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/clients/k8s"
	secretsConfig "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/config"
)

/*
	This struct retrieves Conjur secrets that are required by the pod and pushes them into K8s secrets.
*/
type ProvideConjurSecretsToK8sSecrets struct {
	AccessToken            access_token.AccessToken
	ConjurSecretsRetriever conjur.ConjurSecretsRetriever
	K8sSecretsClient       k8s.K8sSecretsClient
}

func NewProvideConjurSecrets(secretsConfig secretsConfig.Config, AccessToken access_token.AccessToken) (ProvideConjurSecrets *ProvideConjurSecretsToK8sSecrets, err error) {
	k8sSecretsClient, err := k8s.New(secretsConfig)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK017E)
	}

	var conjurSecretsRetriever conjur.ConjurSecretsRetriever

	return &ProvideConjurSecretsToK8sSecrets{
		AccessToken:            AccessToken,
		ConjurSecretsRetriever: conjurSecretsRetriever,
		K8sSecretsClient:       *k8sSecretsClient,
	}, nil
}

func (provideConjurSecretsToK8sSecrets ProvideConjurSecretsToK8sSecrets) Run() error {
	k8sSecretsMap, err := provideConjurSecretsToK8sSecrets.K8sSecretsClient.RetrieveK8sSecrets()
	if err != nil {
		return log.RecordedError(messages.CSPFK021E)
	}

	accessToken, err := provideConjurSecretsToK8sSecrets.AccessToken.Read()
	if err != nil {
		return log.RecordedError(messages.CSPFK002E)
	}

	variableIDs, err := getVariableIDsToRetrieve(k8sSecretsMap.PathMap)
	if err != nil {
		return log.RecordedError(messages.CSPFK037E)
	}

	retrievedConjurSecrets, err := provideConjurSecretsToK8sSecrets.ConjurSecretsRetriever.RetrieveConjurSecrets(accessToken, variableIDs)
	if err != nil {
		return log.RecordedError(messages.CSPFK034E, err.Error())
	}

	err = updateK8sSecretsMapWithConjurSecrets(k8sSecretsMap, retrievedConjurSecrets)
	if err != nil {
		return log.RecordedError(messages.CSPFK027E)
	}

	err = provideConjurSecretsToK8sSecrets.K8sSecretsClient.PatchK8sSecrets(k8sSecretsMap)
	if err != nil {
		return log.RecordedError(messages.CSPFK023E)
	}

	return nil
}

func getVariableIDsToRetrieve(pathMap map[string][]string) ([]string, error) {
	var variableIDs []string

	if len(pathMap) == 0 {
		return nil, log.RecordedError(messages.CSPFK025E)
	}

	for key, _ := range pathMap {
		variableIDs = append(variableIDs, key)
	}

	return variableIDs, nil
}

func updateK8sSecretsMapWithConjurSecrets(k8sSecretsMap *k8s.K8sSecretsMap, conjurSecrets map[string][]byte) error {
	var err error

	// Update K8s map by replacing variable IDs with their corresponding secret values
	for variableId, secret := range conjurSecrets {
		variableId, err = parseVariableID(variableId)
		if err != nil {
			return log.RecordedError(messages.CSPFK035E)
		}

		for _, locationInK8sSecretsMap := range k8sSecretsMap.PathMap[variableId] {
			locationInK8sSecretsMap := strings.Split(locationInK8sSecretsMap, ":")
			k8sSecretName := locationInK8sSecretsMap[0]
			k8sSecretDataEntryKey := locationInK8sSecretsMap[1]
			k8sSecretsMap.K8sSecrets[k8sSecretName][k8sSecretDataEntryKey] = secret
		}
	}

	return nil
}

// The variable ID is in the format "<account>:variable:<variable_id>". we need only the last part.
func parseVariableID(fullVariableId string) (string, error) {
	variableIdParts := strings.Split(fullVariableId, ":")
	if len(variableIdParts) != 3 {
		return "", log.RecordedError(messages.CSPFK036E, fullVariableId)
	}

	return variableIdParts[2], nil
}
