package pushtofile

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const secretGroupPrefix = "conjur.org/conjur-secrets."
const secretGroupPolicyPathPrefix = "conjur.org/conjur-secrets-policy-path."
const secretGroupFileTemplatePrefix = "conjur.org/secret-file-template."
const secretGroupFilePathPrefix = "conjur.org/secret-file-path."
const secretGroupFileFormatPrefix = "conjur.org/secret-file-format."

const defaultFilePermissions os.FileMode = 0664

// Secret represents an application secret that has been retrieved from
// Conjur.
type Secret struct {
	Alias string
	Value string
}

// SecretGroup incorporates all of the information about a secret group
// that has been parsed from that secret group's Annotations.
type SecretGroup struct {
	Name             string
	FilePath         string
	FileTemplate     string
	FileFormat       string
	PolicyPathPrefix string
	FilePermissions  os.FileMode
	SecretSpecs      []SecretSpec
}

// ResolvedSecretSpecs resolves all of the secret paths for a secret
// group by prepending each path with that group's policy path prefix.
func (s *SecretGroup) ResolvedSecretSpecs() []SecretSpec {
	if len(s.PolicyPathPrefix) == 0 {
		return s.SecretSpecs
	}

	specs := make([]SecretSpec, len(s.SecretSpecs))
	copy(specs, s.SecretSpecs)

	// Update specs with policy path prefix
	for i := range specs {
		specs[i].Path = strings.TrimSuffix(s.PolicyPathPrefix, "/") +
			"/" +
			strings.TrimPrefix(specs[i].Path, "/")
	}

	return specs
}

// PushToFile uses the configuration on a secret group to inject secrets into a template
// and write the result to a file.
func (s *SecretGroup) PushToFile(secrets []*Secret) error {
	return s.pushToFileWithDeps(openFileAsWriteCloser, pushToWriter, secrets)
}

func (s *SecretGroup) pushToFileWithDeps(
	depOpenWriteCloser openWriteCloserFunc,
	depPushToWriter pushToWriterFunc,
	secrets []*Secret) error {
	// Make sure all the secret specs are accounted for
	err := validateSecretsAgainstSpecs(secrets, s.SecretSpecs)
	if err != nil {
		return err
	}

	// Determine file template from
	// 1. File template
	// 2. File format
	// 3. Secret specs (user to validate file template)
	fileTemplate, err := maybeFileTemplateFromFormat(
		s.FileTemplate,
		s.FileFormat,
		s.SecretSpecs,
	)
	if err != nil {
		return err
	}

	//// Open and push to file
	wc, err := depOpenWriteCloser(s.FilePath, s.FilePermissions)
	if err != nil {
		return err
	}
	defer func() {
		_ = wc.Close()
	}()

	return depPushToWriter(
		wc,
		s.Name,
		fileTemplate,
		secrets,
	)
}

func validateSecretsAgainstSpecs(
	secrets []*Secret,
	specs []SecretSpec,
) error {
	if len(secrets) != len(specs) {
		return fmt.Errorf(
			"number of secrets (%d) does not match number of secret specs (%d)",
			len(secrets),
			len(specs),
		)
	}

	// Secrets should match SecretSpecs
	var aliasInSecrets = map[string]struct{}{}
	for _, secret := range secrets {
		aliasInSecrets[secret.Alias] = struct{}{}
	}

	var missingAliases []string
	for _, spec := range specs {
		if _, ok := aliasInSecrets[spec.Alias]; !ok {
			missingAliases = append(missingAliases, spec.Alias)
		}
	}

	// Sort strings to ensure deterministic behavior of the method
	sort.Strings(missingAliases)

	if len(missingAliases) > 0 {
		return fmt.Errorf("some secret specs are not present in secrets %q", strings.Join(missingAliases, ""))
	}

	return nil
}

func maybeFileTemplateFromFormat(
	fileTemplate string,
	fileFormat string,
	secretSpecs []SecretSpec,
) (string, error) {
	// One of file format or file template must be set
	if len(fileTemplate)+len(fileFormat) == 0 {
		fileFormat = "yaml"
	}

	// fileFormat is used to set fileTemplate when fileTemplate is not
	// already set
	if len(fileTemplate) == 0 {
		var err error

		fileTemplate, err = FileTemplateForFormat(
			fileFormat,
			secretSpecs,
		)
		if err != nil {
			return "", err
		}
	}

	return fileTemplate, nil
}

// NewSecretGroups creates a collection of secret groups from a map of annotations
func NewSecretGroups(annotations map[string]string) ([]*SecretGroup, []error) {
	var sgs []*SecretGroup

	var errors []error
	for key := range annotations {
		if strings.HasPrefix(key, secretGroupPrefix) {
			groupName := strings.TrimPrefix(key, secretGroupPrefix)
			sg, err := newSecretGroup(annotations, groupName)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			sgs = append(sgs, sg)
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	// Sort secret groups for deterministic order based on group path
	sort.SliceStable(sgs, func(i, j int) bool {
		return sgs[i].Name < sgs[j].Name
	})

	return sgs, nil
}

func newSecretGroup(annotations map[string]string, groupName string) (*SecretGroup, error) {
	groupSecrets := annotations[secretGroupPrefix+groupName]
	secretSpecs, err := NewSecretSpecs([]byte(groupSecrets))
	if err != nil {
		return nil, fmt.Errorf(
			`cannot create secret specs from annotation "%s": %s`,
			secretGroupPrefix+groupName,
			err,
		)
	}

	fileTemplate := annotations[secretGroupFileTemplatePrefix+groupName]
	filePath := annotations[secretGroupFilePathPrefix+groupName]
	fileFormat := annotations[secretGroupFileFormatPrefix+groupName]
	policyPathPrefix := annotations[secretGroupPolicyPathPrefix+groupName]

	if err = validateSecretSpecs(secretSpecs); err != nil {
		return nil, fmt.Errorf(
			"invalid secret specs for group %q: %s",
			groupName,
			err,
		)
	}

	if len(fileFormat) > 0 {
		_, err := FileTemplateForFormat(fileFormat, secretSpecs)
		if err != nil {
			return nil, fmt.Errorf(
				"unable to process file format annotation %q for group %q: %s",
				fileFormat,
				groupName,
				err,
			)
		}
	}

	return &SecretGroup{
		Name:             groupName,
		FilePath:         filePath,
		FileTemplate:     fileTemplate,
		FileFormat:       fileFormat,
		FilePermissions:  defaultFilePermissions,
		PolicyPathPrefix: policyPathPrefix,
		SecretSpecs:      secretSpecs,
	}, nil
}
