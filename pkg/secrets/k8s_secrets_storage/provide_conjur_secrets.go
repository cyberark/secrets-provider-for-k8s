package k8s_secrets_storage

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"gopkg.in/yaml.v2"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/k8s"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
)

type K8sSecretsMap struct {
	// Maps a k8s Secret name to a data-entry map that holds the new entries that will be added to the k8s secret.
	// The data-entry map's key represents an entry name and the value is a Conjur variable ID that holds the value
	// of the required k8s secret. After the secret is retrieved from Conjur we replace the variable ID with its
	// corresponding secret value.
	// The variable ID (which is replaced later with the secret) is held as a byte array so we don't hold the secret as
	// clear text string
	K8sSecrets map[string]map[string][]byte

	// Maps a k8s Secret name to original K8sSecret fetched from k8s
	OriginalK8sSecrets map[string]*v1.Secret

	// Maps a conjur variable id to its place in the k8sSecretsMap. This object helps us to replace
	// the variable IDs with their corresponding secret value in the map
	PathMap map[string][]string
}

/*
	This method is implemented for implementing the ProvideConjurSecrets interface. All that is done here is to
	initialize a K8sSecretsClient and use the internal `run` method.
	That method receives different structs as inputs so they can be mocked.
*/
func ProvideConjurSecretsToK8sSecrets(AccessToken access_token.AccessToken) error {
	config, err := config.NewFromEnv()
	if err != nil {
		return err
	}

	return run(
		k8s.RetrieveK8sSecret,
		k8s.UpdateK8sSecret,
		config.PodNamespace,
		config.RequiredK8sSecrets,
		AccessToken,
		conjur.RetrieveConjurSecrets,
	)
}

func run(retrieveSecretFunc k8s.RetrieveK8sSecretFunc, updateSecretFunc k8s.UpdateK8sSecretFunc, namespace string, requiredK8sSecrets []string, accessToken access_token.AccessToken, retrieveConjurSecretsFunc conjur.RetrieveConjurSecretsFunc) error {
	k8sSecretsMap, err := RetrieveRequiredK8sSecrets(retrieveSecretFunc, namespace, requiredK8sSecrets)

	if err != nil {
		return log.RecordedError(messages.CSPFK021E)
	}

	accessTokenData, err := accessToken.Read()
	if err != nil {
		return log.RecordedError(messages.CSPFK002E)
	}

	variableIDs, err := getVariableIDsToRetrieve(k8sSecretsMap.PathMap)
	if err != nil {
		return log.RecordedError(messages.CSPFK037E)
	}

	retrievedConjurSecrets, err := retrieveConjurSecretsFunc(accessTokenData, variableIDs)
	if err != nil {
		return log.RecordedError(messages.CSPFK034E, err.Error())
	}

	err = updateK8sSecretsMapWithConjurSecrets(k8sSecretsMap, retrievedConjurSecrets)
	if err != nil {
		return log.RecordedError(messages.CSPFK027E)
	}

	err = UpdateRequiredK8sSecrets(updateSecretFunc, namespace, k8sSecretsMap)

	if err != nil {
		return log.RecordedError(messages.CSPFK023E)
	}

	return nil
}

func RetrieveRequiredK8sSecrets(retrieveSecretFunc k8s.RetrieveK8sSecretFunc, namespace string, requiredK8sSecrets []string) (*K8sSecretsMap, error) {
	k8sSecrets := make(map[string]map[string][]byte)
	originalK8sSecrets := make(map[string]*v1.Secret)
	pathMap := make(map[string][]string)

	for _, secretName := range requiredK8sSecrets {

		k8sSecret, err := retrieveSecretFunc(namespace, secretName)
		if err != nil {
			// Error messages returned from K8s should be printed only in debug mode
			log.Debug(messages.CSPFK004D, err.Error())
			return nil, log.RecordedError(messages.CSPFK020E)
		}

		// If K8s secret has no "conjur-map" data entry, return an error
		if _, ok := k8sSecret.Data[config.CONJUR_MAP_KEY]; !ok {
			// Error messages returned from K8s should be printed only in debug mode
			log.Debug(messages.CSPFK008D, secretName, config.CONJUR_MAP_KEY)
			return nil, log.RecordedError(messages.CSPFK028E, secretName)
		}

		// Parse its "conjur-map" data entry and store its values in the new-data-entries map
		// This map holds data entries that will be added to the k8s secret after we retrieve their values from Conjur
		newDataEntriesMap := make(map[string][]byte)
		conjurMap := make(map[string]string)
		for key, value := range k8sSecret.Data {
			if key == config.CONJUR_MAP_KEY {
				if len(value) == 0 {
					// Error messages returned from K8s should be printed only in debug mode
					log.Debug(messages.CSPFK006D, secretName, config.CONJUR_MAP_KEY)
					return nil, log.RecordedError(messages.CSPFK028E, secretName)
				}

				log.Debug(messages.CSPFK009D, config.CONJUR_MAP_KEY, secretName)
				err := yaml.Unmarshal(value, &conjurMap)
				if err != nil {
					// Error messages returned from K8s should be printed only in debug mode
					log.Debug(messages.CSPFK007D, secretName, config.CONJUR_MAP_KEY, err.Error())
					return nil, log.RecordedError(messages.CSPFK028E, secretName)
				} else if conjurMap == nil || len(conjurMap) == 0 {
					// Error messages returned from K8s should be printed only in debug mode
					log.Debug(messages.CSPFK007D, secretName, config.CONJUR_MAP_KEY, "value is empty")
					return nil, log.RecordedError(messages.CSPFK028E, secretName)
				}

				for k8sSecretKey, conjurVariableId := range conjurMap {
					newDataEntriesMap[k8sSecretKey] = []byte(conjurVariableId)

					// This map will help us later to swap the variable id with the secret value
					pathMap[conjurVariableId] = append(pathMap[conjurVariableId], fmt.Sprintf("%s:%s", secretName, k8sSecretKey))
				}
			}
		}

		k8sSecrets[secretName] = newDataEntriesMap
		originalK8sSecrets[secretName] = k8sSecret
	}

	return &K8sSecretsMap{
		K8sSecrets:         k8sSecrets,
		OriginalK8sSecrets: originalK8sSecrets,
		PathMap:            pathMap,
	}, nil
}

func UpdateRequiredK8sSecrets(updateSecretFunc k8s.UpdateK8sSecretFunc, namespace string, k8sSecretsMap *K8sSecretsMap) error {
	for secretName, dataEntryMap := range k8sSecretsMap.K8sSecrets {
		err := updateSecretFunc(namespace, secretName, k8sSecretsMap.OriginalK8sSecrets[secretName], dataEntryMap)
		if err != nil {
			// Error messages returned from K8s should be printed only in debug mode
			log.Debug(messages.CSPFK005D, err.Error())
			return log.RecordedError(messages.CSPFK022E)
		}
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

func updateK8sSecretsMapWithConjurSecrets(k8sSecretsMap *K8sSecretsMap, conjurSecrets map[string][]byte) error {
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
