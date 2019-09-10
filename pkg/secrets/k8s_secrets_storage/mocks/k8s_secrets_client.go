package mocks

import "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/clients/k8s"

// Mocks a K8s secrets client. Uses a mock K8s database to retrieve & patch secrets.
type MockK8sSecretsClient struct {
	MockK8sDB map[string]MockK8sSecret
}

func (k8sSecretsClient MockK8sSecretsClient) RetrieveK8sSecret(_ string, secretName string) (k8s.K8sSecretInterface, error) {
	return k8sSecretsClient.MockK8sDB[secretName], nil
}

func (k8sSecretsClient *MockK8sSecretsClient) PatchK8sSecret(_ string, secretName string, stringDataEntriesMap map[string][]byte) error {
	secretToPatch := k8sSecretsClient.MockK8sDB[secretName]
	for key, value := range stringDataEntriesMap {
		secretToPatch.Data[key] = value
	}

	return nil
}
