package k8ssecretsstorage

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"

	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	k8sClient "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/k8s"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

// Secrets that have been retrieved from Conjur may need to be updated in
// more than one Kubernetes Secrets, and each Kubernetes Secret may refer to
// the application secret with a different name. The updateDestination struct
// represents one destination to which a retrieved Conjur secret value needs
// to be written when Kubernetes Secrets are updated.
type updateDestination struct {
	k8sSecretName string
	secretName    string
	contentType   string
}

type k8sSecretsState struct {
	// Maps a K8s Secret name to the original K8s Secret API object fetched
	// from K8s.
	originalK8sSecrets map[string]*v1.Secret

	// Maps a Conjur variable ID (policy path) to all the updateDestination
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
//   - Retrieving a list of required K8s Secrets
//   - Retrieving all Conjur secrets that are referenced (via variable ID,
//     a.k.a. policy path) by those K8s Secrets.
//   - Updating the K8s Secrets by replacing each Conjur variable ID
//     with the corresponding secret value that was retrieved from Conjur.
type K8sProvider struct {
	k8s                k8sAccessDeps
	conjur             conjurAccessDeps
	log                logDeps
	podNamespace       string
	requiredK8sSecrets []string
	secretsState       k8sSecretsState
	traceContext       context.Context
	sanitizeEnabled    bool
	//prevSecretsChecksums maps a k8s secret name to a sha256 checksum of the
	// corresponding secret content. This is used to detect changes in
	// secret content.
	prevSecretsChecksums map[string]utils.Checksum
	//whether provider runs in repeatable mode or not.
	//In some cases no hard errors are returned but logs to make provider
	//running endlessly
	isRepeatable bool
}

// K8sProviderConfig provides config specific to Kubernetes Secrets provider
type K8sProviderConfig struct {
	PodNamespace       string
	RequiredK8sSecrets []string
	IsRepeatableMode   bool
}

// retrieveSecretLock prevents race condition between ad-hoc secret mutation providing
// and periodic secrets providing
var retrieveSecretLock sync.Mutex

// mutateK8SSecretLock prevents race condition between mutation webhook calls
var mutateK8SSecretLock sync.Mutex

// NewProvider creates a new secret provider for K8s Secrets mode.
func NewProvider(
	traceContext context.Context,
	retrieveConjurSecrets conjur.RetrieveSecretsFunc,
	sanitizeEnabled bool,
	config K8sProviderConfig,
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
		sanitizeEnabled,
		config,
		traceContext)
}

// newProvider creates a new secret provider for K8s Secrets mode
// using dependencies provided for retrieving and updating Kubernetes
// Secrets objects.
func newProvider(
	providerDeps k8sProviderDeps,
	sanitizeEnabled bool,
	config K8sProviderConfig,
	traceContext context.Context,
) K8sProvider {
	return K8sProvider{
		k8s:                providerDeps.k8s,
		conjur:             providerDeps.conjur,
		log:                providerDeps.log,
		podNamespace:       config.PodNamespace,
		requiredK8sSecrets: config.RequiredK8sSecrets,
		sanitizeEnabled:    sanitizeEnabled,
		secretsState: k8sSecretsState{
			originalK8sSecrets: map[string]*v1.Secret{},
			updateDestinations: map[string][]updateDestination{},
		},
		traceContext:         traceContext,
		prevSecretsChecksums: map[string]utils.Checksum{},
		isRepeatable:         config.IsRepeatableMode,
	}
}

// Provide implements a ProviderFunc to retrieve and push secrets to K8s secrets.
// If secrets names is passed as parameter, it override configured secrets to provide
func (p *K8sProvider) Provide(secrets ...string) (bool, error) {
	// Use the global TracerProvider
	tr := trace.NewOtelTracer(otel.Tracer("secrets-provider"))
	// Retrieve required K8s Secrets and parse their Data fields.
	if err := p.retrieveRequiredK8sSecrets(tr, secrets...); err != nil {
		return false, p.log.recordedError(messages.CSPFK021E)
	}

	// Retrieve Conjur secrets for all K8s Secrets.
	var updated bool
	retrievedConjurSecrets, err := p.retrieveConjurSecrets(tr)
	if err != nil {
		// Delete K8s secrets for Conjur variables that no longer exist or the user no longer has permissions to.
		// In the future we'll delete only the secrets that are revoked, but for now we delete all secrets in
		// the group because we don't have a way to determine which secrets are revoked.
		if (strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "404")) && p.sanitizeEnabled {
			updated = true
			rmErr := p.removeDeletedSecrets(tr)
			if rmErr != nil {
				p.log.recordedError(messages.CSPFK063E)
				// Don't return here - continue processing
			}
		}

		if p.isRepeatable {
			p.log.logError(messages.CSPFK034E, err.Error())
			// continue processing
		} else {
			return updated, p.log.recordedError(messages.CSPFK034E, err.Error())
		}
	}

	// Update all K8s Secrets with the retrieved Conjur secrets.
	updated, err = p.updateRequiredK8sSecrets(retrievedConjurSecrets, tr)
	if err != nil {

		if p.isRepeatable {
			p.log.logError(messages.CSPFK023E, err.Error())
			// continue processing
		} else {
			return updated, p.log.recordedError(messages.CSPFK023E)
		}
	}

	// Clear the secrets' state from memory. This prevents leakage of the secret values
	// in `originalK8sSecrets` and prevents `updateDestinations` from growing each time
	// the provider is run (e.g. during rotation).
	p.secretsState = k8sSecretsState{
		originalK8sSecrets: map[string]*v1.Secret{},
		updateDestinations: map[string][]updateDestination{},
	}

	if updated {
		p.log.info(messages.CSPFK009I)
	}
	return updated, nil
}

// Mutate implements a MutateFunc to mutate K8S Secret object catch by webhook.
func (p *K8sProvider) Mutate(secret v1.Secret) (map[string][]byte, error) {
	mutateK8SSecretLock.Lock()
	defer mutateK8SSecretLock.Unlock()

	tr := trace.NewOtelTracer(otel.Tracer("secrets-provider"))
	spanCtx, span := tr.Start(p.traceContext, "Fetch Conjur Secrets")
	defer span.End()

	//prepare clean  data
	p.requiredK8sSecrets = []string{}
	p.secretsState = k8sSecretsState{
		updateDestinations: map[string][]updateDestination{},
	}

	err := p.retrieveRequiredSecrets(secret)

	if err != nil {
		return nil, err
	}

	var variableIDs []string
	updateDests := p.secretsState.updateDestinations
	if updateDests != nil {
		for key := range updateDests {
			variableIDs = append(variableIDs, key)
		}
	}

	if len(variableIDs) == 0 {
		return nil, nil
	}

	retrievedConjurSecrets, err := p.conjur.retrieveSecrets(variableIDs, spanCtx)
	if err != nil {
		p.log.logError(messages.CSPFK070E, secret.Name, err.Error())
		return nil, err
	}

	newSecretsData := p.createSecretData(retrievedConjurSecrets)

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	for itemName, secretValue := range newSecretsData[secret.Name] {
		secret.Data[itemName] = secretValue
	}

	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, "%v", secret.Data)
	checksum, _ := utils.FileChecksum(b)
	p.prevSecretsChecksums[secret.Name] = checksum

	p.log.info(messages.CSPFK026I, secret.Name)

	return newSecretsData[secret.Name], nil
}

func (p *K8sProvider) removeDeletedSecrets(tr trace.Tracer) error {
	log.Info(messages.CSPFK021I)
	emptySecrets := make(map[string][]byte)
	variablesToDelete, err := p.listConjurSecretsToFetch()
	if err != nil {
		return err
	}
	for _, secret := range variablesToDelete {
		emptySecrets[secret] = []byte("")
	}
	_, err = p.updateRequiredK8sSecrets(emptySecrets, tr)
	if err != nil {
		return err
	}
	return nil
}
func (p K8sProvider) Delete(secrets []string) {
	emptySecrets := make(map[string][]byte)
	for _, secret := range secrets {
		delete(p.prevSecretsChecksums, secret)
		emptySecrets[secret] = []byte("")
	}
	for _, secret := range secrets {
		p.deleteK8sSecret(secret)
	}
}

// retrieveRequiredK8sSecrets retrieves all K8s Secrets that need to be
// managed/updated by the Secrets Provider.
func (p *K8sProvider) retrieveRequiredK8sSecrets(tracer trace.Tracer, k8sSecretNames ...string) error {
	spanCtx, span := tracer.Start(p.traceContext, "Gather required K8s Secrets")
	defer span.End()

	if k8sSecretNames == nil || len(k8sSecretNames) == 0 {
		k8sSecretNames = p.requiredK8sSecrets
	}

	for _, k8sSecretName := range k8sSecretNames {
		_, childSpan := tracer.Start(spanCtx, "Retrieve K8s Secret")
		defer childSpan.End()
		if err := p.retrieveRequiredK8sSecret(k8sSecretName); err != nil {
			childSpan.RecordErrorAndSetStatus(err)
			span.RecordErrorAndSetStatus(err)
			if p.isRepeatable {
				//continue processing
				p.log.logError(messages.CSPFK069E, err.Error())
			} else {
				return err
			}
		}
	}
	return nil
}

// retrieveRequiredK8sSecret retrieves an individual K8s Secrets that needs
// to be managed/updated by the Secrets Provider.
func (p *K8sProvider) retrieveRequiredK8sSecret(k8sSecretName string) error {

	// Retrieve the K8s Secret
	k8sSecret, err := p.k8s.retrieveSecret(p.podNamespace, k8sSecretName)
	if err != nil {
		// Error messages returned from K8s should be printed only in debug mode
		p.log.debug(messages.CSPFK004D, err.Error())
		return p.log.recordedError(messages.CSPFK020E)
	}
	if k8sSecret == nil {
		p.log.debug(messages.CSPFK004D, "retrieved object is nil")
		return p.log.recordedError(messages.CSPFK020E)
	}

	return p.retrieveRequiredSecrets(*k8sSecret)
}

// retrieveRequiredSecrets process end parse secrets from K8S secret
func (p *K8sProvider) retrieveRequiredSecrets(k8sSecret v1.Secret) error {

	// Record the K8s Secret API object
	if p.secretsState.originalK8sSecrets == nil {
		p.secretsState.originalK8sSecrets = map[string]*v1.Secret{}
	}
	p.secretsState.originalK8sSecrets[k8sSecret.Name] = &k8sSecret

	// Read the value of the "conjur-map" entry in the K8s Secret's Data
	// field, if it exists. If the entry does not exist or has a null
	// value, return an error.
	conjurMapKey := config.ConjurMapKey
	conjurSecretsYAML, entryExists := k8sSecret.Data[conjurMapKey]
	if !entryExists {
		p.log.debug(messages.CSPFK008D, k8sSecret.Name, conjurMapKey)
		return p.log.recordedError(messages.CSPFK028E, k8sSecret.Name)
	}
	if len(conjurSecretsYAML) == 0 {
		p.log.debug(messages.CSPFK006D, k8sSecret.Name, conjurMapKey)
		return p.log.recordedError(messages.CSPFK028E, k8sSecret.Name)
	}

	// Parse the YAML-formatted Conjur secrets mapping that has been
	// retrieved from this K8s Secret.
	p.log.debug(messages.CSPFK009D, conjurMapKey, k8sSecret.Name)
	return p.parseConjurSecretsYAML(conjurSecretsYAML, k8sSecret.Name)
}

// parseConjurSecretsYAML parses the YAML-formatted Conjur secrets mapping
// that has been retrieved from a K8s Secret.
func (p *K8sProvider) parseConjurSecretsYAML(secretsYAML []byte, k8sSecretName string) error {
	conjurMap := map[string]interface{}{}
	if err := yaml.Unmarshal(secretsYAML, &conjurMap); err != nil {
		p.log.debug(messages.CSPFK007D, k8sSecretName, config.ConjurMapKey, err.Error())
		return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
	}
	if len(conjurMap) == 0 {
		p.log.debug(messages.CSPFK007D, k8sSecretName, config.ConjurMapKey, "value is empty")
		return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
	}
	return p.refreshUpdateDestinations(conjurMap, k8sSecretName)
}

// refreshUpdateDestinations populates the Provider's updateDestinations
// with the Conjur secret variable ID, K8s secret, secret name, and
// content-type as specified in the Conjur secrets mapping.
// The key is an application secret name, the value can be either a
// string (varID) or a map {id: varID (required), content-type: base64 (optional)}.
func (p *K8sProvider) refreshUpdateDestinations(conjurMap map[string]interface{}, k8sSecretName string) error {
	for secretName, contents := range conjurMap {
		switch value := contents.(type) {
		case string:
			dest := updateDestination{k8sSecretName, secretName, "text"}
			p.secretsState.updateDestinations[value] = append(p.secretsState.updateDestinations[value], dest)
		case map[interface{}]interface{}:
			varId, ok := value["id"].(string)
			if !ok || varId == "" {
				return p.log.recordedError(messages.CSPFK037E, secretName, k8sSecretName)
			}

			contentType, ok := value["content-type"].(string)
			if ok && contentType == "base64" {
				dest := updateDestination{k8sSecretName, secretName, "base64"}
				p.secretsState.updateDestinations[varId] = append(p.secretsState.updateDestinations[varId], dest)
				p.log.info(messages.CSPFK022I, secretName, k8sSecretName)
			} else {
				dest := updateDestination{k8sSecretName, secretName, "text"}
				p.secretsState.updateDestinations[varId] = append(p.secretsState.updateDestinations[varId], dest)
			}
		default:
			return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
		}
	}
	return nil
}

func (p *K8sProvider) listConjurSecretsToFetch() ([]string, error) {
	updateDests := p.secretsState.updateDestinations
	if len(updateDests) == 0 {
		return nil, p.log.recordedError(messages.CSPFK025E)
	}

	// Gather the set of variable IDs for all secrets that need to be
	// retrieved from Conjur.
	var variableIDs []string
	for key := range updateDests {
		// If the variable is "*", then we should fetch all secrets
		if key == "*" {
			return []string{"*"}, nil
		}

		variableIDs = append(variableIDs, key)
	}
	return variableIDs, nil
}

func (p *K8sProvider) retrieveConjurSecrets(tracer trace.Tracer) (map[string][]byte, error) {
	retrieveSecretLock.Lock()
	defer retrieveSecretLock.Unlock()

	spanCtx, span := tracer.Start(p.traceContext, "Fetch Conjur Secrets")
	defer span.End()

	variableIDs, err := p.listConjurSecretsToFetch()
	if err != nil {
		return nil, err
	}

	retrievedConjurSecrets, err := p.conjur.retrieveSecrets(variableIDs, spanCtx)
	if err != nil {
		span.RecordErrorAndSetStatus(err)
		if p.isRepeatable {
			p.log.logError(messages.CSPFK034E, err.Error())
		} else {
			return retrievedConjurSecrets, p.log.recordedError(messages.CSPFK034E, err.Error())
		}
	}
	return retrievedConjurSecrets, nil
}

func (p *K8sProvider) updateRequiredK8sSecrets(
	conjurSecrets map[string][]byte, tracer trace.Tracer) (bool, error) {

	var updated = false

	spanCtx, span := tracer.Start(p.traceContext, "Update K8s Secrets")
	defer span.End()

	newSecretData := p.createSecretData(conjurSecrets)

	// Update K8s Secrets with the retrieved Conjur secrets
	for k8sSecretName, secretData := range newSecretData {
		_, childSpan := tracer.Start(spanCtx, "Update K8s Secret")
		defer childSpan.End()
		b := new(bytes.Buffer)
		_, err := fmt.Fprintf(b, "%v", secretData)
		if err != nil {
			p.log.debug(messages.CSPFK005D, err.Error())
			childSpan.RecordErrorAndSetStatus(err)
			return updated, p.log.recordedError(messages.CSPFK022E)
		}

		// Calculate a sha256 checksum on the content
		checksum, _ := utils.FileChecksum(b)

		if utils.ContentHasChanged(k8sSecretName, checksum, p.prevSecretsChecksums) {
			err := p.k8s.updateSecret(
				p.podNamespace,
				k8sSecretName,
				p.secretsState.originalK8sSecrets[k8sSecretName],
				secretData)
			if err != nil {
				// Error messages returned from K8s should be printed only in debug mode
				p.log.debug(messages.CSPFK005D, err.Error())
				childSpan.RecordErrorAndSetStatus(err)
				return false, p.log.recordedError(messages.CSPFK022E)
			}
			p.prevSecretsChecksums[k8sSecretName] = checksum
			updated = true
			p.log.info("%s (%s)", messages.CSPFK009I, k8sSecretName)
		} else {
			p.log.info("%s (%s)", messages.CSPFK020I, k8sSecretName)
			updated = false
		}
	}

	return updated, nil
}

// createSecretData creates a map of entries to be added to the 'Data' fields
// of each K8s Secret. Each entry will map an application secret name to a
// value retrieved from Conjur. If a secret has a 'base64' content type, the
// resulting secret value will be decoded.
func (p *K8sProvider) createSecretData(conjurSecrets map[string][]byte) map[string]map[string][]byte {
	_, isFetchAll := p.secretsState.updateDestinations["*"]

	secretData := map[string]map[string][]byte{}
	for variableID, secretValue := range conjurSecrets {
		dests := p.secretsState.updateDestinations[variableID]
		if isFetchAll {
			// In fetch all mode, add the fetch all destination details
			dests = append(dests, p.secretsState.updateDestinations["*"]...)
		}
		for _, dest := range dests {
			k8sSecretName := dest.k8sSecretName

			// If there are no data entries for this K8s Secret yet, initialize
			// its map of data entries.
			if secretData[k8sSecretName] == nil {
				secretData[k8sSecretName] = map[string][]byte{}
			}

			secretName := dest.secretName
			if secretName == "*" {
				// In fetch all mode, use the Conjur variable ID as the key.
				// However, we need to normalize the key to be a valid K8s secret name.
				secretName = normalizeK8sSecretName(variableID)
				if secretData[k8sSecretName][secretName] != nil {
					// The key already exists. Since the order of the secrets is not guaranteed,
					// this will cause non-deterministic behavior. Log a warning and leave the
					// first value in place.
					p.log.warn(messages.CSPFK067E, secretName)
					continue
				}
			}

			// Check if the secret value should be decoded in this K8s Secret
			if dest.contentType == "base64" {
				decodedSecretValue := make([]byte, base64.StdEncoding.DecodedLen(len(secretValue)))
				_, err := base64.StdEncoding.Decode(decodedSecretValue, secretValue)
				decodedSecretValue = bytes.Trim(decodedSecretValue, "\x00")
				if err != nil {
					// Log the error as a warning but still provide the original secret value
					p.log.warn(messages.CSPFK064E, secretName, dest.contentType, err.Error())
					secretData[k8sSecretName][secretName] = secretValue
				} else {
					secretData[k8sSecretName][secretName] = decodedSecretValue
				}
			} else {
				secretData[k8sSecretName][secretName] = secretValue
			}
		}
	}

	// Check for any variables that were requested explicitly but were not returned by Conjur.
	// This means that the variable does not exist or the user does not have permission to access it.
	// We should set the value of the secret to an empty string.
	if p.sanitizeEnabled {
		// Find updateDestinations that are not in conjurSecrets
		for varID, dests := range p.secretsState.updateDestinations {
			for _, dest := range dests {
				if dest.secretName == "*" {
					// This is a fetch all destination. We need to check all keys in the secret
					// in order to remove any keys that are no longer in Conjur.
					existingKeys := p.secretsState.originalK8sSecrets[dest.k8sSecretName].Data
					for key := range existingKeys {
						if key != config.ConjurMapKey && secretData[dest.k8sSecretName][key] == nil {
							// If the key is not 'conjur-map' and the key is not in the newly
							// fetched secrets, set the value to an empty string. This wipes
							// any old values that are no longer in Conjur. It also has the
							// side effect of wiping any non-conjur secrets. Therefore, we
							// cannot allow non-Conjur secrets in fetch all mode with sanitize enabled.
							secretData[dest.k8sSecretName][key] = []byte("")
						}
					}
					continue
				}

				if conjurSecrets[varID] == nil {
					// The secret does not exist in conjurSecrets, set the value to an empty string
					secretData[dest.k8sSecretName][dest.secretName] = []byte("")
				}
			}
		}
	}

	return secretData
}

func normalizeK8sSecretName(name string) string {
	// Replace any special characters (except ".", "_", and "-") with "."
	regex := regexp.MustCompile(`[^\w\d-_.]`)
	name = regex.ReplaceAllString(name, ".")

	return name
}

// removed all 'k8sSecretName' secret references
func (p K8sProvider) deleteK8sSecret(k8sSecretName string) {

	p.log.debug("Deleting all %s secret references from cache", k8sSecretName)

	if p.secretsState.updateDestinations != nil {
		for key, dests := range p.secretsState.updateDestinations {
			if dests != nil {
				//array of non-delete destination
				remaining := make([]updateDestination, 0)
				for _, dest := range dests {
					if dest.k8sSecretName != k8sSecretName {
						remaining = appendDestination(remaining, dest)
					} else {
						p.log.debug("Deleting %s updateDestinations", dest)
					}
				}
				if len(remaining) > 0 {
					//update destinations array
					p.secretsState.updateDestinations[key] = remaining
				} else {
					//if non-delete is empty, delete whole record from map
					delete(p.secretsState.updateDestinations, key)
				}
			}
		}
	}

	delete(p.prevSecretsChecksums, k8sSecretName)
}

func (p K8sProvider) CHeckContentHasChanged(groupName string, newChecksum utils.Checksum) bool {
	return utils.ContentHasChanged(groupName, newChecksum, p.prevSecretsChecksums)
}

// If not already exists, append new destination to list
func appendDestination(dests []updateDestination, dest updateDestination) []updateDestination {
	for _, destt := range dests {
		if destt.k8sSecretName == dest.k8sSecretName && destt.secretName == dest.secretName {
			return dests
		}
	}
	return append(dests, dest)
}
