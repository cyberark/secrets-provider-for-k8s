package conjur

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// conjurClientWrapper wraps the conjur-api-go Client to provide
// V2 batch retrieval with fallback to V1 for backwards compatibility
type conjurClientWrapper struct {
	client *conjurapi.Client
	useV2  bool
}

// RetrieveBatchSecretsSafe attempts to use V2 batch retrieval API first,
// falling back to V1 if V2 is not available
func (w *conjurClientWrapper) RetrieveBatchSecretsSafe(variableIDs []string) (map[string][]byte, error) {
	if w.useV2 {
		secrets, err := w.retrieveBatchSecretsV2(variableIDs)
		if err != nil {
			if isV2NotAvailableError(err) {
				log.Warn(messages.CSPFK090E, err.Error())
				w.useV2 = false
			} else {
				return nil, err
			}
		} else {
			return secrets, nil
		}
	}

	log.Debug(messages.CSPFK017D, "Using V1 batch retrieval")
	return w.client.RetrieveBatchSecretsSafe(variableIDs)
}

// retrieveBatchSecretsV2 uses the V2 batch retrieval API
func (w *conjurClientWrapper) retrieveBatchSecretsV2(variableIDs []string) (map[string][]byte, error) {
	v2Client := w.client.V2()
	if v2Client == nil {
		return nil, fmt.Errorf("V2 client not available")
	}

	// V2 API expects variable paths without the account:variable: prefix
	// Normalize IDs: "account:variable:path/to/secret" -> "path/to/secret"
	normalizedIDs := make([]string, len(variableIDs))
	originalIDMap := make(map[string]string) // maps normalized ID -> original ID

	for i, id := range variableIDs {
		normalizedID := normalizeVariableIdForV2(id)
		normalizedIDs[i] = normalizedID
		originalIDMap[normalizedID] = id
	}

	batchResp, err := v2Client.BatchRetrieveSecrets(normalizedIDs)
	if err != nil {
		return nil, err
	}

	secrets := make(map[string][]byte)
	var failedSecrets []string

	for _, secret := range batchResp.Secrets {
		if secret.Status == http.StatusOK {
			// Success - map back to original ID format
			originalID := originalIDMap[secret.ID]
			if originalID == "" {
				originalID = secret.ID
			}
			secrets[originalID] = []byte(secret.Value)
		} else {
			// Failed - track for error reporting (use original ID if available)
			originalID := originalIDMap[secret.ID]
			if originalID == "" {
				originalID = secret.ID
			}
			failedSecrets = append(failedSecrets, fmt.Sprintf("%s (status: %d)", originalID, secret.Status))
		}
	}

	if len(failedSecrets) > 0 {
		log.Warn(messages.CSPFK091E, strings.Join(failedSecrets, ", "))
	}

	if len(secrets) == 0 {
		return nil, fmt.Errorf(messages.CSPFK092E)
	}

	log.Info(messages.CSPFK036I, len(secrets))
	return secrets, nil
}

func (w *conjurClientWrapper) Resources(filter *conjurapi.ResourceFilter) ([]map[string]interface{}, error) {
	return w.client.Resources(filter)
}

func (w *conjurClientWrapper) Cleanup() {
	w.client.Cleanup()
}

// isV2NotAvailableError checks if the error indicates V2 API is not available
func isV2NotAvailableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	indicators := []string{
		"not supported in",
		"404",
	}

	for _, indicator := range indicators {
		if strings.Contains(errMsg, indicator) {
			return true
		}
	}
	return false
}

// normalizeVariableIdForV2 converts a variable ID to the format expected by V2 API
// The V2 API expects just the variable path (e.g., "secrets/test_secret")
// not the full identifier format (e.g., "my-account:variable:secrets/test_secret")
func normalizeVariableIdForV2(fullVariableId string) string {
	variableIdParts := strings.SplitN(fullVariableId, ":", 3)
	if len(variableIdParts) == 3 {
		// Format is "account:variable:path" -> return just "path"
		return variableIdParts[2]
	}
	// Already in correct format or unknown format, return as-is
	return fullVariableId
}
