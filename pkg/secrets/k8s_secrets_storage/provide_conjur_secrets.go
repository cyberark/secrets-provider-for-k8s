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
	filetemplates "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/file_templates"
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
	updateDestinations      map[string][]updateDestination
	updateDestinationLookup map[string]map[destinationKey]struct{} // fast dedupe index

}

type destinationKey struct {
	k8sSecretName string
	secretName    string
}

type k8sAccessDeps struct {
	retrieveSecret     k8sClient.RetrieveK8sSecretFunc
	updateSecret       k8sClient.UpdateK8sSecretFunc
	listLabeledSecrets k8sClient.ListLabeledK8sSecretsFunc
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
	// mu protects concurrent access to Provide/ProvideWithCleanup.
	// When both periodic refresh and informer events trigger simultaneously,
	// one goroutine will acquire the lock while the other waits.
	mu sync.Mutex
	// Maps template groups to corresponding K8S Secret
	secretsGroups map[string][]*filetemplates.SecretGroup
}

// K8sProviderConfig provides config specific to Kubernetes Secrets provider
type K8sProviderConfig struct {
	PodNamespace       string
	RequiredK8sSecrets []string
}

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
				k8sClient.ListLabeledK8sSecrets,
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
			originalK8sSecrets:      map[string]*v1.Secret{},
			updateDestinations:      map[string][]updateDestination{},
			updateDestinationLookup: map[string]map[destinationKey]struct{}{},
		},
		traceContext:         traceContext,
		prevSecretsChecksums: map[string]utils.Checksum{},
		secretsGroups:        map[string][]*filetemplates.SecretGroup{},
	}
}

// Provide implements a ProviderFunc to retrieve and push secrets to K8s secrets.
func (p *K8sProvider) Provide() (bool, error) {
	return p.ProvideWithCleanup(map[string][]string{})
}

// ProvideWithCleanup removes specified keys from K8s secrets before updating them with Conjur values.
// keysToRemove maps a K8s Secret name to specific keys that should be removed from a secret.
func (p *K8sProvider) ProvideWithCleanup(keysToRemove map[string][]string) (bool, error) {
	// Acquire lock to prevent concurrent execution.
	// If another goroutine is executing, this will block until it completes.
	p.mu.Lock()
	defer func() {
		// Always clear the secrets' state from memory. This prevents leakage of the secret values
		// in `originalK8sSecrets` and prevents `updateDestinations` from growing each time
		// the provider is run (e.g. during rotation).
		p.secretsState = k8sSecretsState{
			originalK8sSecrets:      map[string]*v1.Secret{},
			updateDestinations:      map[string][]updateDestination{},
			updateDestinationLookup: map[string]map[destinationKey]struct{}{},
		}
		// Also clear secretsGroups to prevent memory growth during rotation
		p.secretsGroups = map[string][]*filetemplates.SecretGroup{}
		// Release lock after all cleanup is done
		p.mu.Unlock()
	}()

	// Use the global TracerProvider
	tr := trace.NewOtelTracer(otel.Tracer("secrets-provider"))
	// Retrieve required K8s Secrets and parse their Data fields.
	if err := p.retrieveRequiredK8sSecrets(tr); err != nil {
		return false, p.log.recordedError(messages.CSPFK021E)
	}

	// In label-based mode with no updateable secrets discovered, return gracefully.
	if len(p.secretsState.updateDestinations) == 0 && len(p.secretsGroups) == 0 && len(keysToRemove) == 0 {
		p.log.warn(messages.CSPFK070E)
		return false, nil
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

		return updated, p.log.recordedError(messages.CSPFK034E, err.Error())
	}

	// Update all K8s Secrets with the retrieved Conjur secrets.
	updated, err = p.updateRequiredK8sSecretsWithCleanup(retrievedConjurSecrets, tr, keysToRemove)
	if err != nil {
		return updated, p.log.recordedError(messages.CSPFK023E)
	}

	p.log.info(messages.CSPFK009I)
	return updated, nil
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

// retrieveRequiredK8sSecrets retrieves all K8s Secrets that need to be
// managed/updated by the Secrets Provider.
func (p *K8sProvider) retrieveRequiredK8sSecrets(tracer trace.Tracer) error {
	spanCtx, span := tracer.Start(p.traceContext, "Gather required K8s Secrets")
	defer span.End()

	// If strict list of secrets is provided from config, process them.
	// Otherwise, try to retrieve and process all labeled secrets.
	if len(p.requiredK8sSecrets) > 0 {
		for _, k8sSecretName := range p.requiredK8sSecrets {
			_, childSpan := tracer.Start(spanCtx, "Retrieve K8s Secret")
			defer childSpan.End()
			k8sSecret, err := p.k8s.retrieveSecret(p.podNamespace, k8sSecretName)
			if err != nil {
				childSpan.RecordErrorAndSetStatus(err)
				span.RecordErrorAndSetStatus(err)
				return p.log.recordedError(messages.CSPFK020E)
			}
			if err := p.retrieveRequiredK8sSecret(k8sSecret, false); err != nil {
				childSpan.RecordErrorAndSetStatus(err)
				span.RecordErrorAndSetStatus(err)
				return err
			}
		}
	} else {
		_, childSpan := tracer.Start(spanCtx, "List Labeled K8s Secrets")
		defer childSpan.End()
		k8sSecrets, err := p.k8s.listLabeledSecrets(p.podNamespace)
		if err != nil {
			childSpan.RecordErrorAndSetStatus(err)
			span.RecordErrorAndSetStatus(err)
			return p.log.recordedError(messages.CSPFK024E)
		}

		for _, k8sSecret := range k8sSecrets.Items {
			_, childSpan := tracer.Start(spanCtx, "Process K8s Secret")
			defer childSpan.End()
			if err := p.retrieveRequiredK8sSecret(&k8sSecret, true); err != nil {
				// In label-based mode, skip invalid secrets and continue processing others
				// instead of failing the entire operation. This makes the provider more
				// resilient to misconfigured secrets.
				childSpan.RecordErrorAndSetStatus(err)
				span.RecordErrorAndSetStatus(err)
				p.log.warn(messages.CSPFK073E, k8sSecret.Name, err.Error())
				// Remove the secret from originalK8sSecrets since we're skipping it
				delete(p.secretsState.originalK8sSecrets, k8sSecret.Name)
				continue
			}
		}
	}
	return nil
}

// retrieveRequiredK8sSecret retrieves an individual K8s Secrets that needs
// to be managed/updated by the Secrets Provider.
//
// Note: allowEmptyConjurMap indicates whether to allow K8s Secrets with
// an empty or missing "conjur-map" entry in their Data field. In label-based
// mode, we allow this so that the provider can process the new labeled secrets
// without conjur-map to remove stale secrets.
func (p *K8sProvider) retrieveRequiredK8sSecret(k8sSecret *v1.Secret, allowEmptyConjurMap bool) error {
	// Record the K8s Secret API object
	p.secretsState.originalK8sSecrets[k8sSecret.Name] = k8sSecret

	// Look for "conjur.org/conjur-secrets.*" annotations and process secret groups
	err := p.processSecretGroupAnnotations(k8sSecret)
	if err != nil {
		return err
	}

	// Parse and process the conjur-map entry if it exists.
	hasValidConjurMap, err := p.parseConjurSecretsYAML(k8sSecret)
	if err != nil {
		return err
	}

	// At least one of "conjur-map" field or "conjur.org/conjur-secrets.*" annotation must be defined.
	// If neither is provided/valid, return an error.
	if !hasValidConjurMap && len(p.secretsGroups) == 0 {
		if allowEmptyConjurMap {
			p.log.info(messages.CSPFK034I, k8sSecret.Name)
		} else {
			conjurMapKey := config.ConjurMapKey
			p.log.logError("At least one of %s data entry or %s annotations must be defined", conjurMapKey, "conjur.org/conjur-secrets.* & conjur.org/secret-file-template.*")
			return p.log.recordedError(messages.CSPFK028E, k8sSecret.Name)
		}
	}

	return nil
}

// processSecretGroupAnnotations processes "conjur.org/conjur-secrets.*" annotations
// and returns a list of SecretGroup objects. Returns an error if any annotation
// is malformed or missing its required template annotation.
func (p *K8sProvider) processSecretGroupAnnotations(k8sSecret *v1.Secret) error {
	var secretGroups []*filetemplates.SecretGroup
	k8sSecretName := k8sSecret.Name

	for annotationName, annotationValue := range k8sSecret.Annotations {
		if !strings.HasPrefix(annotationName, filetemplates.SecretGroupPrefix) {
			continue
		}

		groupName := strings.TrimPrefix(annotationName, filetemplates.SecretGroupPrefix)
		templateAnnotationName := filetemplates.SecretGroupFileTemplatePrefix + groupName

		// Each secret group must have a corresponding template annotation
		if _, tplExists := k8sSecret.Annotations[templateAnnotationName]; !tplExists {
			p.log.warn(messages.CSPFK013D, k8sSecretName, templateAnnotationName)
			continue
		}

		// Parse the secret specs from the annotation value
		secretSpecs, err := filetemplates.NewSecretSpecs([]byte(annotationValue))
		if err != nil {
			p.log.logError(`unable to create secret specs from annotation "%s": %s`, annotationName, err.Error())
			return fmt.Errorf(`unable to create secret specs from annotation "%s": %w`, annotationName, err)
		}
		if len(secretSpecs) == 0 {
			p.log.logError(`annotation "%s" has no secret specs defined`, annotationName)
			return fmt.Errorf(`annotation "%s" has no secret specs defined`, annotationName)
		}

		secretGroup := &filetemplates.SecretGroup{
			Name:        groupName,
			SecretSpecs: secretSpecs,
		}
		secretGroups = append(secretGroups, secretGroup)
	}

	if len(secretGroups) > 0 {
		p.log.debug(messages.CSPFK014D, len(secretGroups), k8sSecretName)
		p.secretsGroups[k8sSecretName] = secretGroups
	}

	return nil
}

// parseConjurSecretsYAML parses the YAML-formatted Conjur secrets mapping
// from a K8s Secret's Data conjur-map field.
// Returns (hasValidConjurMap, error) where:
//   - hasValidConjurMap: true if a valid (non-empty) conjur-map was found and processed
//   - error: only returned if parsing fails and there are no secret groups to fall back on
func (p *K8sProvider) parseConjurSecretsYAML(k8sSecret *v1.Secret) (bool, error) {
	conjurMapKey := config.ConjurMapKey
	k8sSecretName := k8sSecret.Name
	conjurSecretsYAML, conjurMapExists := k8sSecret.Data[conjurMapKey]

	// Log debug messages if conjur-map doesn't exist or is empty
	if !conjurMapExists {
		p.log.debug(messages.CSPFK008D, k8sSecretName, conjurMapKey)
		return false, nil
	}
	if len(conjurSecretsYAML) == 0 {
		p.log.debug(messages.CSPFK006D, k8sSecretName, conjurMapKey)
		return false, nil
	}

	// Parse the YAML-formatted Conjur secrets mapping.
	// An empty map {} serializes to non-empty YAML bytes, so we need to parse it
	// to determine if it's actually empty.
	var conjurMap map[string]interface{}
	if err := yaml.Unmarshal(conjurSecretsYAML, &conjurMap); err != nil {
		p.log.debug(messages.CSPFK007D, k8sSecretName, conjurMapKey, err.Error())
		// If secret groups exist, allow invalid conjur-map (return no error)
		if len(p.secretsGroups) > 0 {
			return false, nil
		}
		return false, p.log.recordedError(messages.CSPFK028E, k8sSecretName)
	}

	// Check if the parsed map is empty
	if len(conjurMap) == 0 {
		p.log.debug(messages.CSPFK007D, k8sSecretName, conjurMapKey, "value is empty")
		return false, nil
	}

	// Process the valid conjur-map
	p.log.debug(messages.CSPFK009D, conjurMapKey, k8sSecretName)
	if err := p.refreshUpdateDestinations(conjurMap, k8sSecretName); err != nil {
		return false, err
	}

	return true, nil
}

// refreshUpdateDestinations populates the Provider's updateDestinations
// with the Conjur secret variable ID, K8s secret, secret name, and
// content-type as specified in the Conjur secrets mapping.
// The key is an application secret name, the value can be either a
// string (varID) or a map {id: varID (required), content-type: base64 (optional)}.
func (p *K8sProvider) refreshUpdateDestinations(conjurMap map[string]interface{}, k8sSecretName string) error {
	for secretName, contents := range conjurMap {
		switch value := contents.(type) {
		case string: //in that case contents is varID
			dest := updateDestination{k8sSecretName, secretName, "text"}
			p.appendDestination(value, dest)
		case map[interface{}]interface{}:
			varId, ok := value["id"].(string)
			if !ok || varId == "" {
				return p.log.recordedError(messages.CSPFK037E, secretName, k8sSecretName)
			}

			contentType, ok := value["content-type"].(string)
			if ok && contentType == "base64" {
				p.log.info(messages.CSPFK022I, secretName, k8sSecretName)
			} else {
				contentType = "text"
			}

			dest := updateDestination{k8sSecretName, secretName, contentType}
			p.appendDestination(varId, dest)

		default:
			return p.log.recordedError(messages.CSPFK028E, k8sSecretName)
		}
	}
	return nil
}

// appendDestination appends a destination to the slice if it doesn't already exist.
// A destination is considered a duplicate if both k8sSecretName and secretName match.
// This prevents duplicate destinations when the same K8s Secret is processed multiple times
// or when the same secret name maps to the same Conjur variable ID. We use an internal
// index to identify duplicates while avoiding O(n) linear search.
func (p *K8sProvider) appendDestination(varID string, dest updateDestination) {
	destLookup := p.secretsState.updateDestinationLookup

	if destLookup[varID] == nil {
		destLookup[varID] = make(map[destinationKey]struct{})
	}

	key := destinationKey{
		k8sSecretName: dest.k8sSecretName,
		secretName:    dest.secretName,
	}

	if _, exists := destLookup[varID][key]; exists {
		return
	}

	destLookup[varID][key] = struct{}{}
	p.secretsState.updateDestinations[varID] = append(p.secretsState.updateDestinations[varID], dest)
}

func (p *K8sProvider) listConjurSecretsToFetch() ([]string, error) {
	updateDests := p.secretsState.updateDestinations

	// If there are no secrets to update, return gracefully.
	if len(updateDests) == 0 && len(p.secretsGroups) == 0 {
		p.log.debug(messages.CSPFK015D)
		return make([]string, 0), nil
	}

	// Gather the set of variable IDs for all secrets that need to be
	// retrieved from Conjur.
	// Use a map to track seen variable IDs for O(1) lookup performance
	seenIDs := make(map[string]bool)
	var variableIDs []string

	for key := range updateDests {
		// If the variable is "*", then we should fetch all secrets
		if key == "*" {
			return []string{"*"}, nil
		}

		if !seenIDs[key] {
			seenIDs[key] = true
			variableIDs = append(variableIDs, key)
		}
	}

	for _, secretGroups := range p.secretsGroups {
		for _, secretGroup := range secretGroups {
			for _, secretSpec := range secretGroup.SecretSpecs {
				if seenIDs[secretSpec.Path] {
					// already added to variableIDs from another source
					continue
				}
				seenIDs[secretSpec.Path] = true
				variableIDs = append(variableIDs, secretSpec.Path)
			}
		}
	}

	if len(variableIDs) == 0 {
		return nil, p.log.recordedError(messages.CSPFK025E)
	}

	return variableIDs, nil
}

func (p *K8sProvider) retrieveConjurSecrets(tracer trace.Tracer) (map[string][]byte, error) {
	spanCtx, span := tracer.Start(p.traceContext, "Fetch Conjur Secrets")
	defer span.End()

	variableIDs, err := p.listConjurSecretsToFetch()
	if err != nil {
		return nil, err
	}

	retrievedConjurSecrets, err := p.conjur.retrieveSecrets(variableIDs, spanCtx)
	if err != nil {
		span.RecordErrorAndSetStatus(err)
		return nil, p.log.recordedError(messages.CSPFK034E, err.Error())
	}
	return retrievedConjurSecrets, nil
}

func (p *K8sProvider) updateRequiredK8sSecrets(
	conjurSecrets map[string][]byte, tracer trace.Tracer) (bool, error) {
	return p.updateRequiredK8sSecretsWithCleanup(conjurSecrets, tracer, map[string][]string{})
}

func (p *K8sProvider) updateRequiredK8sSecretsWithCleanup(
	conjurSecrets map[string][]byte, tracer trace.Tracer, keysToRemove map[string][]string) (bool, error) {

	var updated bool

	spanCtx, span := tracer.Start(p.traceContext, "Update K8s Secrets")
	defer span.End()

	newSecretsDataMap := p.createSecretData(conjurSecrets)
	p.populateGroupTemplateSecretData(conjurSecrets, newSecretsDataMap)

	// Add K8s Secrets that don't have conjur-map (not in newSecretData) to ensure they are processed
	// This handles secrets that were retrieved in retrieveRequiredK8sSecrets but have no conjur-map entries
	// Note: we only need to deal with the ones that need keys removed based on conjur-map changes
	if len(keysToRemove) > 0 {
		for k8sSecretName := range p.secretsState.originalK8sSecrets {
			if newSecretsDataMap[k8sSecretName] == nil && keysToRemove[k8sSecretName] != nil {
				// Secret has entries to be removed but not in newSecretData, add an empty entry so it will be processed
				newSecretsDataMap[k8sSecretName] = map[string][]byte{}
			}
		}
	}

	// Update K8s Secrets with the retrieved Conjur secrets
	for k8sSecretName, secretData := range newSecretsDataMap {
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
			originalSecret := p.secretsState.originalK8sSecrets[k8sSecretName].DeepCopy()

			// Remove keys those are not in conjur-map anymore
			if keysToRemove[k8sSecretName] != nil {
				for _, key := range keysToRemove[k8sSecretName] {
					delete(originalSecret.Data, key)
					p.log.debug(messages.CSPFK033I, key, k8sSecretName)
				}
			}

			err := p.k8s.updateSecret(
				p.podNamespace,
				k8sSecretName,
				originalSecret,
				secretData)
			if err != nil {
				// Error messages returned from K8s should be printed only in debug mode
				p.log.debug(messages.CSPFK005D, err.Error())
				childSpan.RecordErrorAndSetStatus(err)
				return false, p.log.recordedError(messages.CSPFK022E)
			}
			p.prevSecretsChecksums[k8sSecretName] = checksum
			updated = true
		} else {
			p.log.info(messages.CSPFK020I)
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
				n, err := base64.StdEncoding.Decode(decodedSecretValue, secretValue)
				if err != nil {
					// Log the error as a warning but still provide the original secret value
					p.log.warn(messages.CSPFK064E, secretName, dest.contentType, err.Error())
					secretData[k8sSecretName][secretName] = secretValue
				} else {
					secretData[k8sSecretName][secretName] = decodedSecretValue[:n]
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
					if secretData[dest.k8sSecretName] == nil {
						secretData[dest.k8sSecretName] = map[string][]byte{}
					}
					secretData[dest.k8sSecretName][dest.secretName] = []byte("")
				}
			}
		}
	}
	return secretData
}

// GetRemovedKeys compares old and new secrets to find keys that were removed from conjur-map.
// This function assumes both oldSecret and newSecret have the managed-by-provider label set to true,
// as secrets without this label are filtered by the informer and never reach this function.
// Returns a map of K8s Secret names to lists of keys that should be removed.
//
// TODO: if the secret was updated when there is no flag or the flag was disabled, the conjur-map will
// get updated without triggering the onUpdate handler.
// Once the label is enabled again, we won't be able to detect the removed keys anymore.
// In this case, we cannot remove the keys that are not in the new conjur-map any more since we
// won't be able to tell if any key was configured by the user.
// The unexpected keys will be removed the next time the conjur-map is updated while the flag is enabled.
func (p *K8sProvider) GetRemovedKeys(oldSecret, newSecret *v1.Secret) map[string][]string {
	keysToRemove := make(map[string][]string)

	// If sanitization is not enabled, no need to remove any stale secrets
	if !p.sanitizeEnabled {
		return keysToRemove
	}

	if oldSecret == nil || newSecret == nil {
		return keysToRemove
	}

	// Parse the conjur-map from both old and new secrets to detect removed keys
	oldMap := parseConjurMap(oldSecret)
	newMap := parseConjurMap(newSecret)
	removedKeys := diffConjurMapKeys(oldMap, newMap)

	// If any keys were removed from the conjur-map, add them to the cleanup map
	if len(removedKeys) > 0 {
		keysToRemove[newSecret.Name] = removedKeys
		log.Debug(messages.CSPFK032I, newSecret.Name, removedKeys)
	}

	return keysToRemove
}

func parseConjurMap(secret *v1.Secret) map[string]interface{} {
	result := map[string]interface{}{}
	if secret == nil || secret.Data == nil {
		return result
	}

	raw, ok := secret.Data[config.ConjurMapKey]
	if !ok || len(raw) == 0 {
		return result
	}

	_ = yaml.Unmarshal(raw, &result)
	return result
}

func diffConjurMapKeys(oldMap, newMap map[string]interface{}) []string {
	var removed []string
	for key := range oldMap {
		if _, exists := newMap[key]; !exists {
			removed = append(removed, key)
		}
	}
	return removed
}

// populateGroupTemplateSecretData creates a map of entries to be added to the 'Data' fields
// of each K8s Secret. Data fields are created from group secret variables and rendered corresponding group
// template filled with secret values retrieved from Conjur.
// If a secret has a 'base64' content type, the resulting secret value will be decoded.
// This method allows partial success - groups that can be successfully processed will be updated even if other groups fail
func (p *K8sProvider) populateGroupTemplateSecretData(conjurSecrets map[string][]byte,
	newSecretsDataMap map[string]map[string][]byte) error {

	for k8sSecretName, secretGroups := range p.secretsGroups {
		secretsByGroup := map[string][]*filetemplates.Secret{}

		for _, secretGroup := range secretGroups {
			for _, secSpec := range secretGroup.SecretSpecs {
				// Check if the secret value was returned from Conjur
				// If not, log an error and set the value to an empty string
				bValue, ok := conjurSecrets[secSpec.Path]
				if !ok {
					p.log.logError(messages.CSPFK087E, secretGroup.Name, secSpec.Alias)
					bValue = []byte{}
				}

				// Add the retrieved value for the group
				secretsByGroup[secretGroup.Name] = append(
					secretsByGroup[secretGroup.Name],
					&filetemplates.Secret{
						Alias: secSpec.Alias,
						Value: string(bValue),
					})
			}
		}

		// Render each group template
		originalSecret := p.secretsState.originalK8sSecrets[k8sSecretName]
		if originalSecret == nil {
			continue
		}

		for groupName, sec := range secretsByGroup {
			secretsMap := map[string]*filetemplates.Secret{}
			for _, s := range sec {
				secretsMap[s.Alias] = s
			}

			groupTemplate := originalSecret.Annotations[filetemplates.SecretGroupFileTemplatePrefix+groupName]
			tpl, err := filetemplates.GetTemplate(groupName, secretsMap).Parse(groupTemplate)
			if err != nil {
				p.log.logError(messages.CSPFK088E, groupName, k8sSecretName, err.Error())
				continue
			}

			// Render the secret file content
			tplData := filetemplates.TemplateData{
				SecretsArray: sec,
				SecretsMap:   secretsMap,
			}
			fileContent, err := filetemplates.RenderFile(tpl, tplData)
			if err != nil {
				p.log.logError(messages.CSPFK089E, groupName, k8sSecretName, err.Error())
				continue
			}

			if newSecretsDataMap[k8sSecretName] == nil {
				newSecretsDataMap[k8sSecretName] = map[string][]byte{}
			}
			// Use rendered template as the value, groupname as the key for the K8s secret
			newSecretsDataMap[k8sSecretName][groupName] = fileContent.Bytes()
		}
	}
	return nil
}

func normalizeK8sSecretName(name string) string {
	// Replace any special characters (except ".", "_", and "-") with "."
	regex := regexp.MustCompile(`[^\w\d-_.]`)
	name = regex.ReplaceAllString(name, ".")

	return name
}
