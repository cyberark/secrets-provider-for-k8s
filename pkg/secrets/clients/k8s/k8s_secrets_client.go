package k8s

import (
	"fmt"
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
	secretsConfig "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/utils"
)

/*
	This client communicates with K8s to retrieve & patch K8s secrets
*/
type K8sSecretsClient struct {
	Config secretsConfig.Config
}

type K8sSecret struct {
	Secret *v1.Secret
}

type K8sSecretsMap struct {
	// Maps a k8s Secret name to a data-entry map that holds the new entries that will be added to the k8s secret.
	// The data-entry map's key represents an entry name and the value is a Conjur variable ID that holds the value
	// of the required k8s secret. After the secret is retrieved from Conjur we replace the variable ID with its
	// corresponding secret value.
	// The variable ID (which is replaced later with the secret) is held as a byte array so we don't hold the secret as
	// clear text string
	K8sSecrets map[string]map[string][]byte

	// Maps a conjur variable id to its place in the k8sSecretsMap. This object helps us to replace
	// the variable IDs with their corresponding secret value in the map
	PathMap map[string][]string
}

func New(config secretsConfig.Config) (secrets *K8sSecretsClient, err error) {
	return &K8sSecretsClient{
		Config: config,
	}, nil
}

func (k8sSecretsClient K8sSecretsClient) RetrieveK8sSecrets() (*K8sSecretsMap, error) {
	foundConjurMapKey := false
	namespace := k8sSecretsClient.Config.PodNamespace
	requiredK8sSecrets := k8sSecretsClient.Config.RequiredK8sSecrets

	k8sSecrets := make(map[string]map[string][]byte)
	pathMap := make(map[string][]string)

	for _, secretName := range requiredK8sSecrets {

		k8sSecret, err := retrieveK8sSecret(namespace, secretName)
		if err != nil {
			return nil, log.RecordedError(messages.CSPFK020E, err.Error())
		}

		// Parse its "conjur-map" data entry and store its values in the new-data-entries map
		// This map holds data entries that will be added to the k8s secret after we retrieve their values from Conjur
		newDataEntriesMap := make(map[string][]byte)
		for key, value := range k8sSecret.Secret.Data {
			if key == secretsConfig.CONJUR_MAP_KEY {
				if len(value) == 0 {
					return nil, log.RecordedError(messages.CSPFK029E, secretName, secretsConfig.CONJUR_MAP_KEY)
				}
				foundConjurMapKey = true
				// Split the conjur-map to k8s secret keys. each value holds a Conjur variable ID
				conjurMapEntries := strings.Split(string(value), "\n")
				for _, entry := range conjurMapEntries {
					matchedPattern, _ := regexp.MatchString(".+: .+", entry)
					if matchedPattern == false {
						return nil, log.RecordedError(messages.CSPFK030E, secretName, secretsConfig.CONJUR_MAP_KEY)
					}

					// Parse each secret key and store it in the map
					k8sSecretKeyVal := strings.Split(entry, ": ")
					k8sSecretKey := k8sSecretKeyVal[0]
					conjurVariableId := k8sSecretKeyVal[1]
					newDataEntriesMap[k8sSecretKey] = []byte(conjurVariableId)

					// This map will help us later to swap the variable id with the secret value
					pathMap[conjurVariableId] = append(pathMap[conjurVariableId], fmt.Sprintf("%s:%s", secretName, k8sSecretKey))
				}
			}
		}

		// We add the data-entries map to the k8sSecrets map only if the k8s secret has a "conjur-map" data entry
		if len(newDataEntriesMap) > 0 {
			k8sSecrets[secretName] = newDataEntriesMap
		}
	}

	if !foundConjurMapKey {
		return nil, log.RecordedError(messages.CSPFK028E, secretsConfig.CONJUR_MAP_KEY)
	}

	return &K8sSecretsMap{
		K8sSecrets: k8sSecrets,
		PathMap:    pathMap,
	}, nil
}

func (k8sSecretsClient *K8sSecretsClient) PatchK8sSecrets(k8sSecretsMap *K8sSecretsMap) error {
	namespace := k8sSecretsClient.Config.PodNamespace

	for secretName, dataEntryMap := range k8sSecretsMap.K8sSecrets {
		err := patchK8sSecret(namespace, secretName, dataEntryMap)
		if err != nil {
			return log.RecordedError(messages.CSPFK022E, err.Error())
		}
	}

	return nil
}

func configK8sClient() (*kubernetes.Clientset, error) {
	// Create the Kubernetes client
	log.InfoLogger.Printf(messages.CSPFK105I)
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK019E, err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK018E, err.Error())
	}
	// return a K8s client
	return kubeClient, err
}

func retrieveK8sSecret(namespace string, secretName string) (*K8sSecret, error) {
	// get K8s client object
	kubeClient, _ := configK8sClient()
	log.InfoLogger.Printf(messages.CSPFK106I, secretName, namespace)
	k8sSecret, err := kubeClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return &K8sSecret{
		Secret: k8sSecret,
	}, nil
}

func patchK8sSecret(namespace string, secretName string, stringDataEntriesMap map[string][]byte) error {
	// get K8s client object
	kubeClient, _ := configK8sClient()

	stringDataEntry, err := generateStringDataEntry(stringDataEntriesMap)
	if err != nil {
		return log.RecordedError(messages.CSPFK024E)
	}

	log.InfoLogger.Printf(messages.CSPFK107I, secretName, namespace)
	_, err = kubeClient.CoreV1().Secrets(namespace).Patch(secretName, types.StrategicMergePatchType, stringDataEntry)
	// Clear secret from memory
	stringDataEntry = nil
	if err != nil {
		return err
	}

	return nil
}

// Convert the data entry map to a stringData entry for the PATCH request.
// for example, the map:
// {
//   "username": "theuser",
//   "password": "supersecret"
// }
// will be parsed to the stringData entry "{\"stringData\":{\"username\": \"theuser\", \"password\": \"supersecret\"}}"
//
// we need the values to always stay as byte arrays so we don't have Conjur secrets stored as string.
func generateStringDataEntry(stringDataEntriesMap map[string][]byte) ([]byte, error) {
	var entry []byte
	index := 0

	if len(stringDataEntriesMap) == 0 {
		return nil, log.RecordedError(messages.CSPFK026E)
	}

	entries := make([][]byte, len(stringDataEntriesMap))
	// Parse every key-value pair in the map to a "key:value" byte array
	for key, value := range stringDataEntriesMap {
		entry = utils.ByteSlicePrintf(
			`"%v":"%v"`,
			"%v",
			[]byte(key),
			value,
		)
		entries[index] = entry
		index++

		// Clear secret from memory
		value = nil
		entry = nil
	}

	// Add a comma between each pair of entries
	numEntries := len(entries)
	entriesCombined := entries[0]
	for i := range entries {
		if i < numEntries-1 {
			entriesCombined = utils.ByteSlicePrintf(
				`%v,%v`,
				"%v",
				entriesCombined,
				entries[i+1],
			)
		}

		// Clear secret from memory
		entries[i] = nil
	}

	// Wrap all the entries
	stringDataEntry := utils.ByteSlicePrintf(
		`{"stringData":{%v}}`,
		"%v",
		entriesCombined,
	)

	// Clear secret from memory
	entriesCombined = nil

	return stringDataEntry, nil
}
