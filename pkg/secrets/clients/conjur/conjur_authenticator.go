package conjur

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// ConjurAuthenticator defines how to get an access token
type ConjurAuthenticator interface {
	GetAccessToken(ctx context.Context) ([]byte, error)
}

type conjurClient interface {
	IAMAuthenticate() ([]byte, error)
	AzureAuthenticate(string) ([]byte, error)
	GCPAuthenticate(string) ([]byte, error)
}

var conjurGoClient conjurClient
var clientOnce sync.Once

var newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
	c, err := conjurapi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

var authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
	return authenticator.NewAuthenticatorWithAccessToken(cfg, at)
}

// Helper to create the conjur-api-go client for a given authnType for iam, gcp, or azure.
func createConjurClientForAuthenticator(authnURL, authnType string) (conjurClient, error) {
	var createErr error = nil
	clientOnce.Do(func() {
		applianceURL := os.Getenv("CONJUR_APPLIANCE_URL")
		account := os.Getenv("CONJUR_ACCOUNT")
		hostID := os.Getenv("CONJUR_AUTHN_LOGIN")
		sslCert := os.Getenv("CONJUR_SSL_CERTIFICATE")
		if applianceURL == "" || account == "" {
			createErr = fmt.Errorf("CONJUR_APPLIANCE_URL or CONJUR_ACCOUNT environment variable is not set")
			return
		}
		cfg := conjurapi.Config{
			ApplianceURL:      applianceURL,
			Account:           account,
			AuthnType:         authnType,
			CredentialStorage: conjurapi.CredentialStorageNone,
			JWTHostID:         hostID,
			SSLCert:           sslCert,
		}
		if authnType == "iam" || authnType == "azure" {
			serviceID, err := parseServiceID(authnURL, authnType)
			if err != nil {
				createErr = err
				return
			}
			cfg.ServiceID = serviceID
		}
		client, err := newConjurClientFromConfig(cfg)
		if err != nil {
			createErr = fmt.Errorf("%s: %s", messages.CSPFK033E, err.Error())
			return
		}
		conjurGoClient = client
	})
	if createErr != nil {
		return nil, createErr
	}
	return conjurGoClient, nil
}

func parseServiceID(authnURL, authnType string) (string, error) {
	parsedURL, err := url.Parse(authnURL)
	if err != nil {
		return "", fmt.Errorf(messages.CSPFK069E, err)
	}

	pathParts := strings.Split(parsedURL.Path, "/")
	// Remove empty parts
	pathParts = slices.DeleteFunc(pathParts, func(s string) bool { return s == "" })

	// Validate: must end with /authn-{type}/{service_id}
	if len(pathParts) < 2 || pathParts[len(pathParts)-2] != "authn-"+authnType {
		detail := fmt.Sprintf("expected path to end with /authn-%s/<service_id>", authnType)
		return "", fmt.Errorf(messages.CSPFK069E, detail)
	}

	return pathParts[len(pathParts)-1], nil
}

// AuthenticatorFactory defines a function type for creating a ConjurAuthenticator
// implementation given an authenticator config and authn URL.
type AuthenticatorFactory func(authnConfig config.Configuration, authnURL string) (ConjurAuthenticator, error)

// NewAuthenticator is the default authenticator factory that selects the appropriate
// authenticator based on the CONJUR_AUTHN_URL.
func NewAuthenticator(authnConfig config.Configuration, authnURL string) (ConjurAuthenticator, error) {
	log.Debug("Detecting authentication type from URL %q", authnURL)
	switch {
	case strings.Contains(authnURL, "authn-k8s"):
		return NewK8sAuthenticator(authnConfig), nil
	case strings.Contains(authnURL, "authn-jwt"):
		return NewJwtAuthenticator(authnConfig), nil
	case strings.Contains(authnURL, "authn-iam"):
		log.Debug("Using authn-iam")
		return NewIamAuthenticator(authnURL), nil
	case strings.Contains(authnURL, "authn-azure"):
		log.Debug("Using authn-azure")
		return NewAzureAuthenticator(authnURL), nil
	case strings.Contains(authnURL, "authn-gcp"):
		log.Debug("Using authn-gcp")
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
	authn, err := authnNewWithAccessToken(a.authnConfig, accessToken)
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
	authn, err := authnNewWithAccessToken(a.authnConfig, accessToken)
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
	client, err := createConjurClientForAuthenticator(a.authnURL, "iam")
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("%s", messages.CSPFK033E)
	}
	tok, err := client.IAMAuthenticate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", messages.CSPFK010E, err)
	}
	return tok, nil
}

type AzureAuthenticator struct {
	authnURL string
}

func NewAzureAuthenticator(authnURL string) *AzureAuthenticator {
	return &AzureAuthenticator{authnURL: authnURL}
}

func (a *AzureAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	client, err := createConjurClientForAuthenticator(a.authnURL, "azure")
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("%s", messages.CSPFK033E)
	}
	tok, err := client.AzureAuthenticate("")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", messages.CSPFK010E, err)
	}
	return tok, nil
}

type GcpAuthenticator struct {
	authnURL string
}

func NewGcpAuthenticator(authnURL string) *GcpAuthenticator {
	return &GcpAuthenticator{authnURL: authnURL}
}

func (a *GcpAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	client, err := createConjurClientForAuthenticator(a.authnURL, "gcp")
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("%s", messages.CSPFK033E)
	}
	tok, err := client.GCPAuthenticate("")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", messages.CSPFK010E, err)
	}
	return tok, nil
}
