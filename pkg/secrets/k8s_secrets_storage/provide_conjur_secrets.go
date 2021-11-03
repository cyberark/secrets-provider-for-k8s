package k8ssecretsstorage

import (
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	k8sClient "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/k8s"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
)

// Secrets that have been retrieved from Conjur may need to be updated in
// more than one Kubernetes Secrets, and each Kubernetes Secret may refer to
// the application secret with a different name. The updateDestination struct
// represents one destination to which a retrieved Conjur secret value needs
// to be written when Kubernetes Secrets are updated.
type updateDestination struct {
	k8sSecretName string
	secretName    string
}

type k8sSecretsState struct {
	// Maps a K8s Secret name to the original K8s Secret API object fetched
	// from K8s.
	originalK8sSecrets map[string]*v1.Secret

	// Maps a Conjur variable ID (policy path) to all of the updateDestination
	// targets which will need to be updated with the corresponding Conjur
	// secret value after it has been retrieved.
	updateDestinations map[string][]updateDestination
}

type k8sAccessDeps struct {
	retrieveSecret k8sClient.RetrieveK8sSecretFunc
	updateSecret   k8sClient.UpdateK8sSecretFunc
}

type conjurAccessDeps struct {
	retrieveSecrets conjur.RetrieveSecretsFunc
}

type logFunc func(message string, args ...interface{})
type logFuncWithErr func(message string, args ...interface{}) error
type logDeps struct {
	recordedError logFuncWithErr
	logError      logFunc
	warn          logFunc
	info          logFunc
	debug         logFunc
}

type k8sProviderDeps struct {
	k8s    k8sAccessDeps
	conjur conjurAccessDeps
	log    logDeps
}

// K8sProvider is the secret provider to be used for K8s Secrets mode. It
// makes secrets available to applications by:
// - Retrieving a list of required K8s Secrets
// - Retrieving all Conjur secrets that are referenced (via variable ID,
//   a.k.a. policy path) by those K8s Secrets.
// - Updating the K8s Secrets by replacing each Conjur variable ID
//   with the corresponding secret value that was retrieved from Conjur.
type K8sProvider struct {
	k8s                k8sAccessDeps
	conjur             conjurAccessDeps
	log                logDeps
	podNamespace       string
	requiredK8sSecrets []string
	secretsState       k8sSecretsState
}

// NewProvider creates a new secret provider for K8s Secrets mode.
func NewProvider(
	retrieveConjurSecrets conjur.RetrieveSecretsFunc,
	requiredK8sSecrets []string,
	podNamespace string,
) K8sProvider {
	return newProvider(
		k8sProviderDeps{
			k8s: k8sAccessDeps{
				k8sClient.RetrieveK8sSecret,
				k8sClient.UpdateK8sSecret,
			},
			conjur: conjurAccessDeps{
				retrieveConjurSecrets,
			},
			log: logDeps{
				log.RecordedError,
				log.Error,
				log.Warn,
				log.Info,
				log.Debug,
			},
		},
		requiredK8sSecrets,
		podNamespace)
}

// newProvider creates a new secret provider for K8s Secrets mode
// using dependencies provided for retrieving and updating Kubernetes
// Secrets objects.
func newProvider(
	providerDeps k8sProviderDeps,
	requiredK8sSecrets []string,
	podNamespace string,
) K8sProvider {
	return K8sProvider{
		k8s:                providerDeps.k8s,
		conjur:             providerDeps.conjur,
		log:                providerDeps.log,
		podNamespace:       podNamespace,
		requiredK8sSecrets: requiredK8sSecrets,
		secretsState: k8sSecretsState{
			originalK8sSecrets: map[string]*v1.Secret{},
			updateDestinations: map[string][]updateDestination{},
		},
	}
}

// Provide implements a ProviderFunc to retrieve and push secrets to K8s secrets.
func (p K8sProvider) Provide() error {
	// Retrieve required K8s Secrets and parse their Data fields.
	if err := p.retrieveRequiredK8sSecrets(); err != nil {
		return p.log.recordedError(messages.CSPFK021E)
	}

	// Retrieve Conjur secrets for all K8s Secrets.
	retrievedConjurSecrets, err := p.retrieveConjurSecrets()
	if err != nil {
		return p.log.recordedError(messages.CSPFK034E, err.Error())
	}

	// Update all K8s Secrets with the retrieved Conjur secrets.
	if err = p.updateRequiredK8sSecrets(retrievedConjurSecrets); err != nil {
		return p.log.recordedError(messages.CSPFK023E)
	}

	p.log.info(messages.CSPFK009I)
	return nil
}

// retrieveRequiredK8sSecrets retrieves all K8s Secrets that need to be
// managed/updated by the Secrets Provider.
func (p K8sProvider) retrieveRequiredK8sSecrets() error {
	for _, k8sSecretName := range p.requiredK8sSecrets {
		if err := p.retrieveRequiredK8sSecret(k8sSecretName); err != nil {
			return err
		}
	}
	return nil
}

// retrieveRequiredK8sSecret retrieves an individual K8s Secrets that needs
// to be managed/updated by the Secrets Provider.
func (p K8sProvider) retrieveRequiredK8sSecret(k8sSecretName string) error {

	// Retrieve the K8s Secret
	k8sSecret, err := p.k8s.retrieveSecret(p.podNamespace, k8sSecretName)
	if err != nil {
		// Error messages returned from K8s should be printed only in debug mode
		p.log.debug(messages.CSPFK004D, err.Error())
		return p.log.recordedError(messages.CSPFK020E)
	}

	// Record the K8s Secret API object
	p.secretsState.originalK8sSecrets[k8sSecretName] = k8sSecret

	// Read the value of the "conjur-map" entry in the K8s Secret's Data
	// field, if it exists. If the entry does not exist or has a null
	// value, return an error.
	conjurMapKey := config.ConjurMapKey
	conjurSecretsYAML, entryExists := k8sSecret.Data[conjurMapKey]
	if !entryExists {
		p.log.debug(messages.CSPFK008D, k8sSecretName, conjurMapKey)
		return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
	}
	if len(conjurSecretsYAML) == 0 {
		p.log.debug(messages.CSPFK006D, k8sSecretName, conjurMapKey)
		return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
	}

	// Parse the YAML-formatted Conjur secrets mapping that has been
	// retrieved from this K8s Secret.
	p.log.debug(messages.CSPFK009D, conjurMapKey, k8sSecretName)
	return p.parseConjurSecretsYAML(conjurSecretsYAML, k8sSecretName)
}

// Parse the YAML-formatted Conjur secrets mapping that has been retrieved
// from a K8s Secret. This secrets mapping uses application secret names
// as keys and Conjur variable IDs (a.k.a. policy paths) as values.
func (p K8sProvider) parseConjurSecretsYAML(secretsYAML []byte,
	k8sSecretName string) error {

	conjurMap := map[string]string{}
	if err := yaml.Unmarshal(secretsYAML, &conjurMap); err != nil {
		p.log.debug(messages.CSPFK007D, k8sSecretName, config.ConjurMapKey, err.Error())
		return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
	}
	if len(conjurMap) == 0 {
		p.log.debug(messages.CSPFK007D, k8sSecretName, config.ConjurMapKey, "value is empty")
		return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
	}

	for secretName, varID := range conjurMap {
		dest := updateDestination{k8sSecretName, secretName}
		p.secretsState.updateDestinations[varID] =
			append(p.secretsState.updateDestinations[varID], dest)
	}

	return nil
}

func (p K8sProvider) retrieveConjurSecrets() (map[string][]byte, error) {
	updateDests := p.secretsState.updateDestinations
	if len(updateDests) == 0 {
		return nil, p.log.recordedError(messages.CSPFK025E)
	}

	// Gather the set of variable IDs for all secrets that need to be
	// retrieved from Conjur.
	var variableIDs []string
	for key := range updateDests {
		variableIDs = append(variableIDs, key)
	}

	retrievedConjurSecrets, err := p.conjur.retrieveSecrets(variableIDs)
	if err != nil {
		return nil, p.log.recordedError(messages.CSPFK034E, err.Error())
	}
	return retrievedConjurSecrets, nil
}

func (p K8sProvider) updateRequiredK8sSecrets(
	conjurSecrets map[string][]byte) error {

	// Create a map of entries to be added to the 'Data' fields of each
	// K8s Secret. Each entry will map an application secret name to
	// a value retrieved from Conjur.
	newSecretData := map[string]map[string][]byte{}
	for variableID, secretValue := range conjurSecrets {
		dests := p.secretsState.updateDestinations[variableID]
		for _, dest := range dests {
			k8sSecretName := dest.k8sSecretName
			secretName := dest.secretName
			// If there are no data entries for this K8s Secret yet, initialize
			// its map of data entries.
			if newSecretData[k8sSecretName] == nil {
				newSecretData[k8sSecretName] = map[string][]byte{}
			}
			newSecretData[k8sSecretName][secretName] = secretValue
		}
		// Null out the secret value
		conjurSecrets[variableID] = []byte{}
	}

	// Update K8s Secrets with the retrieved Conjur secrets
	for k8sSecretName, secretData := range newSecretData {
		err := p.k8s.updateSecret(
			p.podNamespace,
			k8sSecretName,
			p.secretsState.originalK8sSecrets[k8sSecretName],
			secretData)
		if err != nil {
			// Error messages returned from K8s should be printed only in debug mode
			p.log.debug(messages.CSPFK005D, err.Error())
			return p.log.recordedError(messages.CSPFK022E)
		}
	}

	return nil
}
