package k8s

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/utils"
)

// This interface is used to mock a K8sSecretsClient struct
type K8sSecretsClientInterface interface {
	RetrieveK8sSecret(namespace string, secretName string) (K8sSecretInterface, error)
	PatchK8sSecret(namespace string, secretName string, stringDataEntriesMap map[string][]byte) error
}

// This empty struct represents a Kubernetes Secrets client. It is used to retrieve & patch k8s secrets.
type K8sSecretsClient struct{}

func configK8sClient() (*kubernetes.Clientset, error) {
	// Create the Kubernetes client
	log.Info(messages.CSPFK004I)
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		// Error messages returned from K8s should be printed only in debug mode
		log.Debug(messages.CSPFK002D, err.Error())
		return nil, log.RecordedError(messages.CSPFK019E)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		// Error messages returned from K8s should be printed only in debug mode
		log.Debug(messages.CSPFK003D, err.Error())
		return nil, log.RecordedError(messages.CSPFK018E)
	}
	// return a K8s client
	return kubeClient, err
}

func (k8sSecretsClient K8sSecretsClient) RetrieveK8sSecret(namespace string, secretName string) (K8sSecretInterface, error) {
	// get K8s client object
	kubeClient, _ := configK8sClient()
	log.Info(messages.CSPFK005I, secretName, namespace)
	k8sSecret, err := kubeClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return &K8sSecret{
		Secret: k8sSecret,
	}, nil
}

func (k8sSecretsClient K8sSecretsClient) PatchK8sSecret(namespace string, secretName string, stringDataEntriesMap map[string][]byte) error {
	// get K8s client object
	kubeClient, _ := configK8sClient()

	stringDataEntry, err := generateStringDataEntry(stringDataEntriesMap)
	if err != nil {
		return log.RecordedError(messages.CSPFK024E)
	}

	log.Info(messages.CSPFK006I, secretName, namespace)
	_, err = kubeClient.CoreV1().Secrets(namespace).Patch(secretName, types.StrategicMergePatchType, stringDataEntry)
	// Clear secret from memory
	stringDataEntry = nil
	if err != nil {
		return err
	}

	return nil
}

/*
	Convert the data entry map to a stringData entry for the PATCH request.
	for example, the map:
	{
	  "username": "theuser",
	  "password": "supersecret"
	}
	will be parsed to the stringData entry "{\"stringData\":{\"username\": \"theuser\", \"password\": \"supersecret\"}}"

	we need the values to always stay as byte arrays so we don't have Conjur secrets stored as string.
*/
func generateStringDataEntry(stringDataEntriesMap map[string][]byte) ([]byte, error) {
	var entry []byte
	index := 0

	if len(stringDataEntriesMap) == 0 {
		return nil, log.RecordedError(messages.CSPFK026E)
	}

	entries := make([][]byte, len(stringDataEntriesMap))
	// Parse every key-value pair in the map to a "key:value" byte array
	for key, value := range stringDataEntriesMap {
		value = escapedSecret(value)
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

// Escape secrets with backslashes
// Otherwise, patching K8s secrets will fail because backslashes in Conjur secret are not escaped
func escapedSecret(secretByte []byte) []byte {
	return bytes.ReplaceAll(secretByte, []byte("\\"), []byte("\\\\"))
}
