package mocks

import (
	"errors"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/clients/k8s"
)

// Mocks a K8s secrets client. Uses a mock K8s database to retrieve & patch secrets.
type MockK8sSecretsClient struct {
	Permissions map[string]bool
}

func (k8sSecretsClient MockK8sSecretsClient) RetrieveK8sSecret(_ string, secretName string) (k8s.K8sSecretInterface, error) {
	if !k8sSecretsClient.Permissions["get"] {
		return nil, errors.New("custom error")
	}
	return MockK8sDB[secretName], nil
}

func (k8sSecretsClient *MockK8sSecretsClient) PatchK8sSecret(_ string, secretName string, stringDataEntriesMap map[string][]byte) error {
	if !k8sSecretsClient.Permissions["patch"] {
		return errors.New("custom error")
	}

	secretToPatch := MockK8sDB[secretName]
	for key, value := range stringDataEntriesMap {
		secretToPatch.Data[key] = value
	}

	return nil
}
