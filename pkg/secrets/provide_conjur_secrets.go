package secrets

import (
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
)

type ProvideConjurSecrets func(AccessToken access_token.AccessToken, config *config.Config) error

func GetProvideConjurSecretFunc(storeType string) (ProvideConjurSecrets, error) {
	var provideConjurSecretFunc ProvideConjurSecrets
	if storeType == config.K8S {
		provideConjurSecretFunc = k8s_secrets_storage.ProvideConjurSecretsToK8sSecrets
	}

	return provideConjurSecretFunc, nil
}
