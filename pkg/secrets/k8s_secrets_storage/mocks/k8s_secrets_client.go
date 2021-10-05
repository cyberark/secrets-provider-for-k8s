package mocks

import (
	"errors"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
)

type KubeSecretsMockClient struct {
	// Mocks a K8s database. Maps k8s secret names to mock K8s secrets.
	Database map[string]map[string][]byte
	// TODO: CanRetrieve and CanUpdate are really just used to assert on the presence of errors
	// 	and should probably just be an optional error.
	CanRetrieve bool
	CanUpdate   bool
}

func NewKubeSecretsMockClient() KubeSecretsMockClient {
	client := KubeSecretsMockClient{
		Database:    map[string]map[string][]byte{},
		CanRetrieve: true,
		CanUpdate:   true,
	}

	return client
}

func (c KubeSecretsMockClient) AddSecret(
	secretName string,
	key string,
	keyConjurPath string,
) {
	conjurMap := map[string]string{
		key: keyConjurPath,
	}
	conjurMapBytes, err := yaml.Marshal(conjurMap)
	if err != nil {
		panic(err)
	}

	c.Database[secretName] = map[string][]byte{
		"conjur-map": conjurMapBytes,
	}
}

func (c KubeSecretsMockClient) RetrieveSecret(_ string, secretName string) (*v1.Secret, error) {
	if !c.CanRetrieve {
		return nil, errors.New("custom error")
	}

	// Check if the secret exists in the mock K8s DB
	secretData, ok := c.Database[secretName]
	if !ok {
		return nil, errors.New("custom error")
	}

	return &v1.Secret{
		Data: secretData,
	}, nil
}

func (c KubeSecretsMockClient) UpdateSecret(_ string, secretName string, originalK8sSecret *v1.Secret, stringDataEntriesMap map[string][]byte) error {
	if !c.CanUpdate {
		return errors.New("custom error")
	}

	secretToUpdate := c.Database[secretName]
	for key, value := range stringDataEntriesMap {
		secretToUpdate[key] = value
	}

	return nil
}
