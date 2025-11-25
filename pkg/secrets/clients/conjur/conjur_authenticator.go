package conjur

import (
	"context"
	"fmt"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// ConjurAuthenticator defines how to get an access token
type ConjurAuthenticator interface {
	GetAccessToken(ctx context.Context) ([]byte, error)
}

// AuthenticatorFactory defines a function type for creating a ConjurAuthenticator
// implementation given an authenticator config and authn URL.
type AuthenticatorFactory func(authnConfig config.Configuration, authnURL string) (ConjurAuthenticator, error)

// NewAuthenticator is the default authenticator factory that selects the appropriate
// authenticator based on the CONJUR_AUTHN_URL.
func NewAuthenticator(authnConfig config.Configuration, authnURL string) (ConjurAuthenticator, error) {
	switch {
	case strings.Contains(authnURL, "authn-k8s"):
		return NewK8sAuthenticator(authnConfig), nil
	case strings.Contains(authnURL, "authn-jwt"):
		return NewJwtAuthenticator(authnConfig), nil
	case strings.Contains(authnURL, "authn-iam"):
		return NewIamAuthenticator(authnURL), nil
	case strings.Contains(authnURL, "authn-azure"):
		return NewAzureAuthenticator(authnURL), nil
	case strings.Contains(authnURL, "authn-gcp"):
		return NewGcpAuthenticator(authnURL), nil
	default:
		return nil, fmt.Errorf("unsupported authenticator in CONJUR_AUTHN_URL: %s", authnURL)
	}
}

// K8sAuthenticator uses conjur-authn-k8s-client for authn-k8s
type K8sAuthenticator struct {
	authnConfig config.Configuration
}

func NewK8sAuthenticator(authnConfig config.Configuration) *K8sAuthenticator {
	return &K8sAuthenticator{authnConfig: authnConfig}
}

func (a *K8sAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	accessToken, err := memory.NewAccessToken()
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK001E)
	}
	authn, err := authenticator.NewAuthenticatorWithAccessToken(a.authnConfig, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK009E)
	}
	if err := authn.AuthenticateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK010E)
	}
	tokenData, err := authn.GetAccessToken().Read()
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK002E)
	}
	result := make([]byte, len(tokenData))
	copy(result, tokenData)
	defer authn.GetAccessToken().Delete()
	for i := range tokenData {
		tokenData[i] = 0
	}

	return result, nil
}

// JwtAuthenticator uses conjur-authn-k8s-client for authn-jwt
type JwtAuthenticator struct {
	authnConfig config.Configuration
}

func NewJwtAuthenticator(authnConfig config.Configuration) *JwtAuthenticator {
	return &JwtAuthenticator{authnConfig: authnConfig}
}

func (a *JwtAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	accessToken, err := memory.NewAccessToken()
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK001E)
	}
	authn, err := authenticator.NewAuthenticatorWithAccessToken(a.authnConfig, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK009E)
	}
	if err := authn.AuthenticateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK010E)
	}
	tokenData, err := authn.GetAccessToken().Read()
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK002E)
	}

	result := make([]byte, len(tokenData))
	copy(result, tokenData)
	defer authn.GetAccessToken().Delete()
	for i := range tokenData {
		tokenData[i] = 0
	}

	return result, nil
}

type IamAuthenticator struct {
	authnURL string
}

func NewIamAuthenticator(authnURL string) *IamAuthenticator {
	return &IamAuthenticator{authnURL: authnURL}
}

func (a *IamAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	// TODO
	return nil, fmt.Errorf("This authentication not yet implemented")
}

type AzureAuthenticator struct {
	authnURL string
}

func NewAzureAuthenticator(authnURL string) *AzureAuthenticator {
	return &AzureAuthenticator{authnURL: authnURL}
}

func (a *AzureAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	// TODO
	return nil, fmt.Errorf("This authentication not yet implemented")
}

type GcpAuthenticator struct {
	authnURL string
}

func NewGcpAuthenticator(authnURL string) *GcpAuthenticator {
	return &GcpAuthenticator{authnURL: authnURL}
}

func (a *GcpAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	// TODO
	return nil, fmt.Errorf("This authentication not yet implemented")
}
