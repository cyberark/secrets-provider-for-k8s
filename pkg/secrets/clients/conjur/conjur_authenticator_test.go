package conjur

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/stretchr/testify/require"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// --- Mocks used across tests -----------------------------------------------------

type mockAuthenticator struct {
	at *memory.AccessToken
}

func (f *mockAuthenticator) Authenticate() error                               { return nil }
func (f *mockAuthenticator) AuthenticateWithContext(ctx context.Context) error { return nil }
func (f *mockAuthenticator) GetAccessToken() access_token.AccessToken          { return f.at }

type mockConjurClient struct {
	token []byte
}

func (f *mockConjurClient) InternalAuthenticate() ([]byte, error) { return f.token, nil }

type failAuth struct{ at *memory.AccessToken }

func (f *failAuth) Authenticate() error { return nil }
func (f *failAuth) AuthenticateWithContext(ctx context.Context) error {
	return fmt.Errorf("authenticate fail")
}
func (f *failAuth) GetAccessToken() access_token.AccessToken { return f.at }

type failingConjurClient struct{}

func (f *failingConjurClient) InternalAuthenticate() ([]byte, error) {
	return nil, fmt.Errorf("auth fail")
}

// --- parseServiceID tests -------------------------------------------------------

func TestParseServiceID_IAM(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expectedID  string
		shouldError bool
	}{
		{"Edge with /api", "https://edge.example.com/api/authn-iam/my-service", "my-service", false},
		{"No /api", "https://conjur.example.com/authn-iam/my-service", "my-service", false},
		{"Proxy custom path", "https://proxy.example.com/custom/path/authn-iam/my-service", "my-service", false},
		{"Trailing slash", "https://conjur.example.com/authn-iam/my-service/", "my-service", false},
		{"Multiple slashes", "https://conjur.example.com//authn-iam//my-service", "my-service", false},
		{"With port", "https://conjur.example.com:8443/authn-iam/my-service", "my-service", false},
		{"Missing service id", "https://conjur.example.com/authn-iam", "", true},
		{"Wrong auth type", "https://conjur.example.com/authn-jwt/my-service", "", true},
		{"Malformed URL", "://invalid-url", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id, err := parseServiceID(tc.url, "iam")
			if tc.shouldError {
				require.Error(t, err)
				require.Empty(t, id)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedID, id)
		})
	}
}

func TestParseServiceID_Azure(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expectedID  string
		shouldError bool
	}{
		{"Basic", "https://conjur.example.com/authn-azure/azure-service", "azure-service", false},
		{"With /api", "https://edge.example.com/api/authn-azure/azure-service", "azure-service", false},
		{"Missing service id", "https://conjur.example.com/authn-azure", "", true},
		{"Wrong auth type", "https://conjur.example.com/authn-iam/my-service", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id, err := parseServiceID(tc.url, "azure")
			if tc.shouldError {
				require.Error(t, err)
				require.Empty(t, id)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedID, id)
		})
	}
}

// --- createConjurClientForAuthenticator tests ----------------------------------------------------

func TestCreateConjurClientForAuthenticator_MissingEnv(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "")
	t.Setenv("CONJUR_ACCOUNT", "")
	t.Setenv("CONJUR_AUTHN_LOGIN", "")

	client, err := createConjurClientForAuthenticator("https://conjur.example.com/authn-iam/iam-service", "iam")
	require.Error(t, err, "expected createConjurClientForAuthenticator to return an error when env vars are missing; got client=%v", client)
	require.Contains(t, err.Error(), messages.CSPFK033E, "expected error to contain CSPFK033E error code")
}

func TestCreateConjurClientForAuthenticator_InvalidServiceID(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	_, err := createConjurClientForAuthenticator("https://conjur.example.com/authn-iam", "iam")
	require.Error(t, err)
	require.Contains(t, err.Error(), "CSPFK069E")
}

func TestCreateConjurClientForAuthenticator_HappyPath_GCP(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	oldFactory := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return &mockConjurClient{}, nil
	}
	t.Cleanup(func() { newConjurClientFromConfig = oldFactory })

	client, err := createConjurClientForAuthenticator("https://conjur.example.com/authn-gcp", "gcp")
	require.NoError(t, err)
	require.NotNil(t, client)
}

// --- NewAuthenticator factory tests ------------------------------------------------

func TestNewAuthenticatorFactory_SelectsCorrectTypes(t *testing.T) {
	cases := []struct {
		url      string
		expected reflect.Type
	}{
		{"https://conjur.example.com/authn-k8s/some", reflect.TypeOf(&K8sAuthenticator{})},
		// Note: authn-jwt is excluded from this test because it requires a physical JWT token file
		// at /var/run/secrets/kubernetes.io/serviceaccount/token. JWT authenticator is tested separately.
		{"https://conjur.example.com/authn-iam/some", reflect.TypeOf(&IamAuthenticator{})},
		{"https://conjur.example.com/authn-azure/some", reflect.TypeOf(&AzureAuthenticator{})},
		{"https://conjur.example.com/authn-gcp", reflect.TypeOf(&GcpAuthenticator{})},
	}

	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			customEnv := func(key string) string {
				switch key {
				case "CONJUR_AUTHN_URL":
					return tc.url
				case "MY_POD_NAME":
					return "test-pod"
				case "MY_POD_NAMESPACE":
					return "test-namespace"
				case "CONJUR_AUTHN_LOGIN":
					return "host/test"
				case "CONJUR_ACCOUNT":
					return "test-account"
				case "CONJUR_APPLIANCE_URL":
					return "https://conjur.example.com"
				case "CONJUR_SSL_CERTIFICATE":
					return "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"
				default:
					return ""
				}
			}
			authn, err := NewAuthenticator(customEnv)
			require.NoError(t, err)
			require.NotNil(t, authn)
			require.Equal(t, tc.expected, reflect.TypeOf(authn))
		})
	}
}

func TestNewAuthenticatorFactory_Unsupported(t *testing.T) {
	customEnv := func(key string) string {
		if key == "CONJUR_AUTHN_URL" {
			return "https://conjur.example.com/unsupported/abc"
		}
		return ""
	}
	_, err := NewAuthenticator(customEnv)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported authenticator")
}

func TestNewAuthenticatorFactory_InvalidAuthnIdentity(t *testing.T) {
	customEnv := func(key string) string {
		switch key {
		case "CONJUR_AUTHN_URL":
			return "https://conjur.example.com/authn-k8s/test-service"
		case "MY_POD_NAME":
			return "test-pod"
		case "MY_POD_NAMESPACE":
			return "test-namespace"
		case "CONJUR_AUTHN_LOGIN":
			return "1" // Invalid: doesn't start with 'host/'
		case "CONJUR_ACCOUNT":
			return "test-account"
		case "CONJUR_APPLIANCE_URL":
			return "https://conjur.example.com"
		case "CONJUR_SSL_CERTIFICATE":
			return "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"
		default:
			return ""
		}
	}
	_, err := NewAuthenticator(customEnv)
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK008E, "expected CSPFK008E error when authn-identity is invalid")
}

// --- K8s / JWT error-path tests -------------------------------------------------
func TestK8sAuthenticator_GetAccessToken_Error_AuthnConstructorFails(t *testing.T) {
	old := authnNewWithAccessToken
	authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
		return nil, fmt.Errorf("constructor failure")
	}
	t.Cleanup(func() { authnNewWithAccessToken = old })

	var authnConfig config.Configuration
	auth := NewK8sAuthenticator(authnConfig)
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK009E)
}

func TestK8sAuthenticator_GetAccessToken_Error_AuthenticateFails(t *testing.T) {
	old := authnNewWithAccessToken
	authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
		return &failAuth{at: at}, nil
	}
	t.Cleanup(func() { authnNewWithAccessToken = old })

	var authnConfig config.Configuration
	auth := NewK8sAuthenticator(authnConfig)
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK010E)
}

func TestJwtAuthenticator_GetAccessToken_Error_AuthnConstructorFails(t *testing.T) {
	old := authnNewWithAccessToken
	authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
		return nil, fmt.Errorf("constructor failure")
	}
	t.Cleanup(func() { authnNewWithAccessToken = old })

	var authnConfig config.Configuration
	auth := NewJwtAuthenticator(authnConfig)
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK009E)
}

func TestJwtAuthenticator_GetAccessToken_Error_AuthenticateFails(t *testing.T) {
	old := authnNewWithAccessToken
	authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
		return &failAuth{at: at}, nil
	}
	t.Cleanup(func() { authnNewWithAccessToken = old })

	var authnConfig config.Configuration
	auth := NewJwtAuthenticator(authnConfig)
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK010E)
}

// --- K8s / JWT success tests ------------------

func TestK8sAuthenticator_GetAccessToken_Success(t *testing.T) {
	old := authnNewWithAccessToken
	authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
		_ = at.Write([]byte("k8s-token"))
		return &mockAuthenticator{at: at}, nil
	}
	t.Cleanup(func() { authnNewWithAccessToken = old })

	var authnConfig config.Configuration
	auth := NewK8sAuthenticator(authnConfig)
	tok, err := auth.GetAccessToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, []byte("k8s-token"), tok)
}

func TestJwtAuthenticator_GetAccessToken_Success(t *testing.T) {
	old := authnNewWithAccessToken
	authnNewWithAccessToken = func(cfg config.Configuration, at *memory.AccessToken) (authenticator.Authenticator, error) {
		_ = at.Write([]byte("jwt-token"))
		return &mockAuthenticator{at: at}, nil
	}
	t.Cleanup(func() { authnNewWithAccessToken = old })

	var authnConfig config.Configuration
	auth := NewJwtAuthenticator(authnConfig)
	tok, err := auth.GetAccessToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, []byte("jwt-token"), tok)
}

// --- IAM / Azure / GCP client-error tests --------------------------------------

func TestIamAuthenticator_GetAccessToken_ClientError(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return nil, fmt.Errorf("factory failure")
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewIamAuthenticator("https://conjur.example.com/authn-iam/iam-service")
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK033E)
}

func TestIamAuthenticator_GetAccessToken_NilClientProducesCSPFK033E(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		// simulate factory that returns nil client but no error
		return nil, nil
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewIamAuthenticator("https://conjur.example.com/authn-iam/iam-service")
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK033E)
}

func TestAzureAuthenticator_GetAccessToken_ClientError(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return nil, fmt.Errorf("factory failure")
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewAzureAuthenticator("https://conjur.example.com/authn-azure/azure-service")
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK033E)
}

func TestAzureAuthenticator_GetAccessToken_NilClientProducesCSPFK033E(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return nil, nil
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewAzureAuthenticator("https://conjur.example.com/authn-azure/azure-service")
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK033E)
}

func TestGcpAuthenticator_GetAccessToken_ClientError(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return nil, fmt.Errorf("factory failure")
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewGcpAuthenticator("https://conjur.example.com/authn-gcp")
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK033E)
}

func TestGcpAuthenticator_GetAccessToken_NilClientProducesCSPFK033E(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return nil, nil
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewGcpAuthenticator("https://conjur.example.com/authn-gcp")
	_, err := auth.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), messages.CSPFK033E)
}

// --- IAM / Azure / GCP success tests (override newConjurClientFromConfig seam) ------------

func TestIamAuthenticator_GetAccessToken_Success(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return &mockConjurClient{token: []byte("iam-token")}, nil
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewIamAuthenticator("https://conjur.example.com/authn-iam/iam-service")
	tok, err := auth.GetAccessToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, []byte("iam-token"), tok)
}

func TestAzureAuthenticator_GetAccessToken_Success(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return &mockConjurClient{token: []byte("azure-token")}, nil
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewAzureAuthenticator("https://conjur.example.com/authn-azure/azure-service")
	tok, err := auth.GetAccessToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, []byte("azure-token"), tok)
}

func TestGcpAuthenticator_GetAccessToken_Success(t *testing.T) {
	t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
	t.Setenv("CONJUR_ACCOUNT", "test")

	old := newConjurClientFromConfig
	newConjurClientFromConfig = func(cfg conjurapi.Config) (conjurClient, error) {
		return &mockConjurClient{token: []byte("gcp-token")}, nil
	}
	t.Cleanup(func() { newConjurClientFromConfig = old })

	auth := NewGcpAuthenticator("https://conjur.example.com/authn-gcp")
	tok, err := auth.GetAccessToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, []byte("gcp-token"), tok)
}
