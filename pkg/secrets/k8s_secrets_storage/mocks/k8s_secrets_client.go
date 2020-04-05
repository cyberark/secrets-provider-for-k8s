package mocks

import (
	"errors"
	v1 "k8s.io/api/core/v1"
)

var CanGetK8sSecrets bool
var CanUpdateK8sSecrets bool

func RetrieveK8sSecret(_ string, secretName string) (*v1.Secret, error) {
	if !CanGetK8sSecrets {
		return nil, errors.New("custom error")
	}

	// Check if the secret exists in the mock K8s DB
	if _, ok := MockK8sDB[secretName]; !ok {
		return nil, errors.New("custom error")
	}

	return &v1.Secret{
		Data: MockK8sDB[secretName],
	}, nil
}

func UpdateK8sSecret(_ string, secretName string, originalK8sSecret *v1.Secret, stringDataEntriesMap map[string][]byte) error {
	if !CanUpdateK8sSecrets {
		return errors.New("custom error")
	}

	secretToUpdate := MockK8sDB[secretName]
	for key, value := range stringDataEntriesMap {
		secretToUpdate[key] = value
	}

	return nil
}
