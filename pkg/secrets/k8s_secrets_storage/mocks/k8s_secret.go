package mocks

type MockK8sSecret struct {
	Data map[string][]byte
}

func (k8sSecret MockK8sSecret) GetSecretData() map[string][]byte {
	return k8sSecret.Data
}
