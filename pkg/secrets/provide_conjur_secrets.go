package secrets

import "github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"

type ProvideConjurSecrets func(AccessToken access_token.AccessToken) error
