package conjur

import (
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

type RetrieveConjurSecretsFunc func(accessToken []byte, variableIDs []string) (map[string][]byte, error)

func RetrieveConjurSecrets(accessToken []byte, variableIDs []string) (map[string][]byte, error) {
	log.Info(messages.CSPFK003I, variableIDs)

	conjurClient, err := NewConjurClient(accessToken)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK033E)
	}

	retrievedSecretsByFullIDs, err := conjurClient.RetrieveBatchSecrets(variableIDs)
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
