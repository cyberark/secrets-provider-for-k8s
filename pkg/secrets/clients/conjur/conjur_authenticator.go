package conjur

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

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
	InternalAuthenticate() ([]byte, error)
}

var newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
	switch cfg.AuthnType {
	case "iam":
		return conjurapi.NewClientFromAWSCredentials(cfg, telemetryData)
	case "azure":
		return conjurapi.NewClientFromAzureCredentials(cfg, telemetryData)
	case "gcp":
		return conjurapi.NewClientFromGCPCredentials(cfg, "", telemetryData)
	default:
		return conjurapi.NewClient(cfg, telemetryData)
	}
}

var authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
	return authenticator.NewAuthenticatorWithAccessToken(cfg, at)
}

// Helper to create the conjur-api-go client for a given authnType for iam, gcp, or azure.
func createConjurClientForAuthenticator(authnURL, authnType string) (conjurClient, error) {
	cfg := conjurapi.Config{
		ApplianceURL:      os.Getenv("CONJUR_APPLIANCE_URL"),
		Account:           os.Getenv("CONJUR_ACCOUNT"),
		JWTHostID:         os.Getenv("CONJUR_AUTHN_LOGIN"),
		SSLCert:           os.Getenv("CONJUR_SSL_CERTIFICATE"),
		AuthnType:         authnType,
		CredentialStorage: conjurapi.CredentialStorageNone,
	}
	if authnType == "iam" || authnType == "azure" {
		serviceID, err := parseServiceID(authnURL, authnType)
		if err != nil {
			return nil, err
		}
		cfg.ServiceID = serviceID
	}
	client, err := newConjurClientFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", messages.CSPFK033E, err.Error())
	}
	return client, nil
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

// EnvFunc defines a function type for retrieving environment variables or annotations
type EnvFunc func(string) string

// AuthenticatorFactory defines a function type for creating a ConjurAuthenticator
// implementation given a customEnv function for reading config.
type AuthenticatorFactory func(customEnv EnvFunc) (ConjurAuthenticator, error)

// NewAuthenticator is the default authenticator factory that selects the appropriate
// authenticator based on the environment configuration.
// If CONJUR_AUTHN_API_KEY is set, plain authn (API key) mode is used.
// Otherwise, the authenticator is selected based on the CONJUR_AUTHN_URL.
func NewAuthenticator(customEnv EnvFunc) (ConjurAuthenticator, error) {
	// If CONJUR_AUTHN_API_KEY is set, use plain authn mode
	apiKey := os.Getenv("CONJUR_AUTHN_API_KEY")
	if apiKey != "" {
		log.Info(messages.CSPFK042I)
		applianceURL := os.Getenv("CONJUR_APPLIANCE_URL")
		account := os.Getenv("CONJUR_ACCOUNT")
		login := customEnv("CONJUR_AUTHN_LOGIN")
		if applianceURL == "" {
			return nil, fmt.Errorf(messages.CSPFK004E, "CONJUR_APPLIANCE_URL")
		}
		if account == "" {
			return nil, fmt.Errorf(messages.CSPFK004E, "CONJUR_ACCOUNT")
		}
		if login == "" {
			return nil, fmt.Errorf(messages.CSPFK004E, "CONJUR_AUTHN_LOGIN")
		}
		return NewPlainAuthenticator(applianceURL, account, login, apiKey), nil
	}

	authnURL := customEnv("CONJUR_AUTHN_URL")
	log.Debug("Detecting authentication type from URL %q", authnURL)
	switch {
	case strings.Contains(authnURL, "authn-k8s"), strings.Contains(authnURL, "authn-jwt"):
		// Load config using conjur-authn-k8s-client for authn-k8s and authn-jwt
		authnConfig, err := config.NewConfigFromCustomEnv(os.ReadFile, customEnv)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", messages.CSPFK008E, err)
		}
		if strings.Contains(authnURL, "authn-k8s") {
			return NewK8sAuthenticator(authnConfig), nil
		}
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
	return readToken(client)
}

func readToken(client conjurClient) ([]byte, error) {
	token, err := client.InternalAuthenticate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", messages.CSPFK010E, err)
	}
	return token, nil
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
	return readToken(client)
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
	return readToken(client)
}

// PlainAuthenticator uses plain authn (host ID + API key) to authenticate with Conjur.
// It POSTs the API key to /authn/{account}/{url-encoded-login}/authenticate
// and returns the raw JSON access token from the response body.
type PlainAuthenticator struct {
	applianceURL string
	account      string
	login        string
	apiKey       string
}

func NewPlainAuthenticator(applianceURL, account, login, apiKey string) *PlainAuthenticator {
	return &PlainAuthenticator{
		applianceURL: applianceURL,
		account:      account,
		login:        login,
		apiKey:       apiKey,
	}
}

func (a *PlainAuthenticator) GetAccessToken(ctx context.Context) ([]byte, error) {
	// Build the authn URL: {applianceURL}/authn/{account}/{url-encoded-login}/authenticate
	encodedLogin := url.PathEscape(a.login)
	authnURL := fmt.Sprintf("%s/authn/%s/%s/authenticate",
		strings.TrimRight(a.applianceURL, "/"),
		url.PathEscape(a.account),
		encodedLogin,
	)

	log.Debug(messages.CSPFK017D, authnURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authnURL, strings.NewReader(a.apiKey))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", messages.CSPFK010E, err)
	}
	req.Header.Set("Content-Type", "text/plain")

	httpClient, err := newHTTPClientWithConjurSSL()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", messages.CSPFK010E, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", messages.CSPFK010E, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read response body: %w", messages.CSPFK010E, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: authn returned HTTP %d: %s", messages.CSPFK010E, resp.StatusCode, string(body))
	}

	log.Debug(messages.CSPFK018D)
	return body, nil
}

// newHTTPClientWithConjurSSL creates an HTTP client that trusts the CA certificate
// provided via CONJUR_SSL_CERTIFICATE (cert content) or CONJUR_CERT_FILE (path to cert file).
// Same resolution logic as conjur-authn-k8s-client's common.ReadSSLCert:
//   - CONJUR_SSL_CERTIFICATE takes precedence (cert content directly)
//   - CONJUR_CERT_FILE is the fallback (path to a PEM file, e.g. /etc/ssl/certs/ca-certificates.crt)
//   - If neither is set, returns an error
func newHTTPClientWithConjurSSL() (*http.Client, error) {
	sslCert := os.Getenv("CONJUR_SSL_CERTIFICATE")
	certFile := os.Getenv("CONJUR_CERT_FILE")

	var certPEM []byte
	switch {
	case sslCert != "":
		certPEM = []byte(sslCert)
	case certFile != "":
		var err error
		certPEM, err = os.ReadFile(certFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CONJUR_CERT_FILE %q: %w", certFile, err)
		}
	default:
		return nil, fmt.Errorf(messages.CSPFK004E, "CONJUR_SSL_CERTIFICATE or CONJUR_CERT_FILE")
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		return nil, fmt.Errorf("failed to parse certificate from provided SSL configuration")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
		Proxy: http.ProxyFromEnvironment,
	}

	return &http.Client{Transport: transport}, nil
}
