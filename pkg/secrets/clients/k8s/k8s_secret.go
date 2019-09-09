package k8s

import (
	v1 "k8s.io/api/core/v1"
)

// This interface is used to mock a K8sSecret struct
type K8sSecretInterface interface {
	GetSecretData() map[string][]byte
}

type K8sSecret struct {
	Secret *v1.Secret
}

func (k8sSecret K8sSecret) GetSecretData() map[string][]byte {
	return k8sSecret.Secret.Data
}
