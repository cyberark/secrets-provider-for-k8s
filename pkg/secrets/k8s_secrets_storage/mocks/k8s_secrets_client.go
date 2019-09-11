package mocks

import (
	"errors"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/clients/k8s"
)

var CanGetK8sSecrets bool
var CanPatchK8sSecrets bool

var RetrieveK8sSecret k8s.RetrieveK8sSecretFunc = func(_ string, secretName string) (map[string][]byte, error) {
	if !CanGetK8sSecrets {
		return nil, errors.New("custom error")
	}

	// Check if the secret exists in the mock K8s DB
	if _, ok := MockK8sDB[secretName]; !ok {
		return nil, errors.New("custom error")
	}

	return MockK8sDB[secretName], nil
}

var PatchK8sSecret k8s.PatchK8sSecretFunc = func(_ string, secretName string, stringDataEntriesMap map[string][]byte) error {
	if !CanPatchK8sSecrets {
		return errors.New("custom error")
	}

	secretToPatch := MockK8sDB[secretName]
	for key, value := range stringDataEntriesMap {
		secretToPatch[key] = value
	}

	return nil
}
