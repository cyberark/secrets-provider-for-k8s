package secrets

/*
	Structs implementing this interface provide Conjur secrets to a storage. For example, ProvideConjurSecretsToK8sSecrets
	retrieves Conjur secrets that are required by the pod and pushes them into K8s secrets.
*/
type ProvideConjurSecretsInterface interface {
	Run() error
}
