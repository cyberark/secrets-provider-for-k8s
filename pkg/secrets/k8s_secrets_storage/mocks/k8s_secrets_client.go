package mocks

import (
	"errors"
)

var CanGetK8sSecrets bool
var CanPatchK8sSecrets bool

func RetrieveK8sSecret(_ string, secretName string) (map[string][]byte, error) {
	if !CanGetK8sSecrets {
		return nil, errors.New("custom error")
	}

	// Check if the secret exists in the mock K8s DB
	if _, ok := MockK8sDB[secretName]; !ok {
		return nil, errors.New("custom error")
	}

	return MockK8sDB[secretName], nil
}

func PatchK8sSecret(_ string, secretName string, stringDataEntriesMap map[string][]byte) error {
	if !CanPatchK8sSecrets {
		return errors.New("custom error")
	}

	secretToPatch := MockK8sDB[secretName]
	for key, value := range stringDataEntriesMap {
		secretToPatch[key] = value
	}

	return nil
}
