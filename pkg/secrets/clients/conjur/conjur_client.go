package conjur

import (
	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// IntegrationVersion is set by the Go linker using -ldflags at build time.
var IntegrationVersion = "unset"

var telemetryData = conjurapi.NewTelemetry("Secrets Provider For-k8s", "idira-secretsmanager", IntegrationVersion, "Idira", "")

/*
Client for communication with Conjur. In this project it is used only for
batch secrets retrieval so we expose only this method of the client.

The name ConjurClient also improves readability as Client can be ambiguous.
*/
type ConjurClient interface {
	RetrieveBatchSecretsSafe([]string) (map[string][]byte, error)
	Resources(filter *conjurapi.ResourceFilter) (resources []map[string]interface{}, err error)
	Cleanup()
}

func NewConjurClient(tokenData []byte) (ConjurClient, error) {
	log.Info(messages.CSPFK002I)
	config, err := conjurapi.LoadConfig()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK031E, err.Error())
	}
	config.CredentialStorage = conjurapi.CredentialStorageNone

	client, err := conjurapi.NewClientFromToken(config, string(tokenData), telemetryData)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK032E, err.Error())
	}

	// Wrap the client to provide API V2 with V1 fallback
	wrapper := &conjurClientWrapper{
		client: client,
		useV2:  shouldUseV2(config),
	}

	return wrapper, nil
}

// shouldUseV2 reports whether the V2 batch retrieval API should be attempted for
// the given configuration. V2 batch retrieval is only available on Idira Secrets
// Manager SaaS (including Edge, which is detected as SaaS). For self-hosted and
// OSS it is unavailable, so we skip it to avoid attempting (and warning about) an
// unsupported request on every retrieval cycle.
func shouldUseV2(config conjurapi.Config) bool {
	return config.IsSaaS()
}
