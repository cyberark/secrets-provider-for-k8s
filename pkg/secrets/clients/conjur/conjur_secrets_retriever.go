package conjur

import (
	"context"
	"fmt"
	"strings"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

var fetchAllMaxSecrets = 500

// SecretRetriever implements a Retrieve function that is capable of
// authenticating with Conjur and retrieving multiple Conjur variables
// in bulk.
type secretRetriever struct {
	authn authenticator.Authenticator
}

// RetrieveSecretsFunc defines a function type for retrieving secrets.
type RetrieveSecretsFunc func(variableIDs []string, traceContext context.Context) (map[string][]byte, error)

// RetrieverFactory defines a function type for creating a RetrieveSecretsFunc
// implementation given an authenticator config.
type RetrieverFactory func(authnConfig config.Configuration) (RetrieveSecretsFunc, error)

// NewSecretRetriever creates a new SecretRetriever and Authenticator
// given an authenticator config.
func NewSecretRetriever(authnConfig config.Configuration) (RetrieveSecretsFunc, error) {
	accessToken, err := memory.NewAccessToken()
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK001E)
	}

	authn, err := authenticator.NewAuthenticatorWithAccessToken(authnConfig, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%s", messages.CSPFK009E)
	}

	return secretRetriever{
		authn: authn,
	}.Retrieve, nil
}

// Retrieve implements a RetrieveSecretsFunc for a given SecretRetriever.
// Authenticates the client, and retrieves a given batch of variables from Conjur.
func (retriever secretRetriever) Retrieve(variableIDs []string, traceContext context.Context) (map[string][]byte, error) {
	authn := retriever.authn

	err := authn.AuthenticateWithContext(traceContext)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK010E)
	}

	accessTokenData, err := authn.GetAccessToken().Read()
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK002E)
	}
	// Always delete the access token. The deletion is idempotent and never fails
	defer authn.GetAccessToken().Delete()
	defer func() {
		// Clear the access token from memory after we use it to authenticate
		for b := range accessTokenData {
			accessTokenData[b] = 0
		}
	}()

	// Determine whether to fetch all secrets or a specific list
	fetchAll := len(variableIDs) == 1 && variableIDs[0] == "*"

	tr := trace.NewOtelTracer(otel.Tracer("secrets-provider"))
	_, span := tr.Start(traceContext, "Retrieve secrets")
	span.SetAttributes(attribute.Bool("fetch_all", fetchAll))
	if !fetchAll {
		span.SetAttributes(attribute.Int("variable_count", len(variableIDs)))
	}
	defer span.End()

	conjurClient, err := NewConjurClient(accessTokenData)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK033E)
	}

	if fetchAll {
		return retrieveConjurSecretsAll(conjurClient)
	}

	return retrieveConjurSecrets(conjurClient, variableIDs)
}

func retrieveConjurSecrets(conjurClient ConjurClient, variableIDs []string) (map[string][]byte, error) {
	log.Debug(messages.CSPFK003I, variableIDs)

	if len(variableIDs) == 0 {
		return nil, log.RecordedError(messages.CSPFK034E, "no variables to retrieve")
	}

	retrievedSecretsByFullIDs, err := conjurClient.RetrieveBatchSecretsSafe(variableIDs)
	if err != nil {
		return nil, err
	}

	// Normalise secret IDs from batch secrets back to <variable_id>
	var retrievedSecrets = map[string][]byte{}
	for id, secret := range retrievedSecretsByFullIDs {
		retrievedSecrets[normaliseVariableId(id)] = secret
		delete(retrievedSecretsByFullIDs, id)
	}

	return retrievedSecrets, nil
}

func retrieveConjurSecretsAll(conjurClient ConjurClient) (map[string][]byte, error) {
	log.Info(messages.CSPFK023I)

	// Page through all secrets available to the host
	allResourcePaths := []string{}
	for offset := 0; ; offset += 100 {
		resFilter := &conjurapi.ResourceFilter{
			Kind:   "variable",
			Limit:  100,
			Offset: offset,
		}
		resources, err := conjurClient.Resources(resFilter)
		if err != nil {
			return nil, err
		}

		log.Debug(messages.CSPFK010D, len(resources))

		for _, candidate := range resources {
			allResourcePaths = append(allResourcePaths, candidate["id"].(string))
		}

		// If we have less than 100 resources, we reached the last page
		if len(resources) < 100 {
			break
		}

		// Limit the maximum number of secrets we can fetch to prevent DoS
		if len(allResourcePaths) >= fetchAllMaxSecrets {
			log.Warn(messages.CSPFK066E, fetchAllMaxSecrets)
			break
		}
	}

	if len(allResourcePaths) == 0 {
		return nil, log.RecordedError(messages.CSPFK034E, "no variables to retrieve")
	}

	log.Info(messages.CSPFK003I, allResourcePaths)

	// Retrieve all secrets in a single batch
	retrievedSecretsByFullIDs, err := conjurClient.RetrieveBatchSecretsSafe(allResourcePaths)
	if err != nil {
		return nil, err
	}

	// Normalise secret IDs from batch secrets back to <variable_id>
	var retrievedSecrets = map[string][]byte{}
	for id, secret := range retrievedSecretsByFullIDs {
		retrievedSecrets[normaliseVariableId(id)] = secret
		delete(retrievedSecretsByFullIDs, id)
	}

	return retrievedSecrets, nil
}

// The variable ID can be in the format "<account>:variable:<variable_id>". This function
// just makes sure that if a variable is of the form "<account>:variable:<variable_id>"
// we normalise it to "<variable_id>", otherwise we just leave it be!
func normaliseVariableId(fullVariableId string) string {
	variableIdParts := strings.SplitN(fullVariableId, ":", 3)
	if len(variableIdParts) == 3 {
		return variableIdParts[2]
	}

	return fullVariableId
}
