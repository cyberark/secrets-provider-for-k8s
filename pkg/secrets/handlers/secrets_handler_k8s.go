package handlers

import (
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
	secretsConfig "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/conjur"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/k8s"
)

type SecretsHandlerK8sUseCase struct {
	AccessToken            access_token.AccessToken
	ConjurSecretsRetriever conjur.ConjurSecretsRetriever
	K8sSecretsHandler      k8s.K8sSecretsHandler
}

func NewSecretHandlerK8sUseCase(secretsConfig secretsConfig.Config, AccessToken access_token.AccessToken) (SecretsHandler *SecretsHandlerK8sUseCase, err error) {
	k8sSecretsHandler, err := k8s.New(secretsConfig)
	if err != nil {
		return nil, log.RecorderError(messages.CSPFK017E)
	}

	var conjurSecretsRetriever conjur.ConjurSecretsRetriever

	return &SecretsHandlerK8sUseCase{
		AccessToken:            AccessToken,
		ConjurSecretsRetriever: conjurSecretsRetriever,
		K8sSecretsHandler:      *k8sSecretsHandler,
	}, nil
}

func (secretsHandlerK8sUseCase SecretsHandlerK8sUseCase) HandleSecrets() error {
	k8sSecretsMap, err := secretsHandlerK8sUseCase.K8sSecretsHandler.RetrieveK8sSecrets()
	if err != nil {
		return log.RecorderError(messages.CSPFK021E)
	}

	accessToken, err := secretsHandlerK8sUseCase.AccessToken.Read()
	if err != nil {
		return log.RecorderError(messages.CSPFK002E)
	}

	variableIDs, err := getVariableIDsToRetrieve(k8sSecretsMap.PathMap)
	if err != nil {
		return log.RecorderError(messages.CSPFK037E)
	}

	retrievedConjurSecrets, err := secretsHandlerK8sUseCase.ConjurSecretsRetriever.RetrieveConjurSecrets(accessToken, variableIDs)
	if err != nil {
		return log.RecorderError(messages.CSPFK034E, err.Error())
	}

	err = updateK8sSecretsMapWithConjurSecrets(k8sSecretsMap, retrievedConjurSecrets)
	if err != nil {
		return log.RecorderError(messages.CSPFK027E)
	}

	err = secretsHandlerK8sUseCase.K8sSecretsHandler.PatchK8sSecrets(k8sSecretsMap)
	if err != nil {
		return log.RecorderError(messages.CSPFK023E)
	}

	return nil
}

func getVariableIDsToRetrieve(pathMap map[string][]string) ([]string, error) {
	var variableIDs []string

	if len(pathMap) == 0 {
		return nil, log.RecorderError(messages.CSPFK025E)
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
			return log.RecorderError(messages.CSPFK035E)
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
		return "", log.RecorderError(messages.CSPFK036E, fullVariableId)
	}

	return variableIdParts[2], nil
}
