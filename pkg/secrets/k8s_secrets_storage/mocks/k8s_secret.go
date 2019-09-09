package mocks

type K8sSecret struct {
	Data map[string][]byte
}

func (k8sSecret K8sSecret) GetSecretData() map[string][]byte {
	return k8sSecret.Data
}
