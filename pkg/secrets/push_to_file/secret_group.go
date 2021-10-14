package push_to_file

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

const defaultFilePermissions os.FileMode = 0660

type Secret struct {
	Alias string
	Value string
}

type SecretGroup struct {
	Name            string
	FilePath        string
	FileTemplate    string
	FileFormat      string
	PolicyPathPrefix      string
	FilePermissions os.FileMode
	SecretSpecs     []SecretSpec
}

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
	return s.pushToFileWithDeps(pushToWriter, openFileToWriterCloser, secrets)
}

func (s *SecretGroup) pushToFileWithDeps(
	pushToWriter toWriterPusher,
	openWriteCloser toWriteCloserOpener,
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
	wc, err := openWriteCloser(s.FilePath, s.FilePermissions)
	if err != nil {
		return err
	}
	defer func() {
		_ = wc.Close()
	}()

	return pushToWriter(
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
		return "", fmt.Errorf("%s", `missing one of "file template" or "file format" for group`)
	}

	// fileTemplate is only modified when
	// 1. fileTemplate is not set. fileTemplate takes precedence
	// 2. fileFormat is set
	if len(fileTemplate) == 0 && len(fileFormat) > 0 {
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
func NewSecretGroups(annotations map[string]string) ([]*SecretGroup, error) {
	var sgs []*SecretGroup

	// TODO: perhaps accumulate errors
	for k, v := range annotations {
		if strings.HasPrefix(k, secretGroupPrefix) {
			groupName := strings.TrimPrefix(k, secretGroupPrefix)
			secretSpecs, err := NewSecretSpecs([]byte(v))
			if err != nil {
				return nil, fmt.Errorf(
					"unable to create secret specs from annotation %q: %s",
						k,
						err,
					)
				continue
			}

			fileTemplate := annotations[secretGroupFileTemplatePrefix+groupName]
			filePath := annotations[secretGroupFilePathPrefix+groupName]
			fileFormat := annotations[secretGroupFileFormatPrefix+groupName]
			policyPathPrefix := annotations[secretGroupPolicyPathPrefix+groupName]

			if len(fileFormat) > 0 {
				_, err := FileTemplateForFormat(fileFormat, secretSpecs)
				if err != nil {
					return nil, fmt.Errorf(
						`unable to process file format annotation %q for group: %s`,
						fileFormat,
						err,
					)
					continue
				}
			}

			sgs = append(sgs, &SecretGroup{
				Name:            groupName,
				FilePath:        filePath,
				FileTemplate:    fileTemplate,
				FileFormat:      fileFormat,
				FilePermissions: defaultFilePermissions,
				PolicyPathPrefix: policyPathPrefix,
				SecretSpecs:     secretSpecs,
			})
		}
	}

	// Sort secret groups for deterministic order based on group path
	sort.SliceStable(sgs, func(i, j int) bool {
		return sgs[i].Name < sgs[j].Name
	})

	return sgs, nil
}