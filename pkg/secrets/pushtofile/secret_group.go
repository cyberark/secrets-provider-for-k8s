package pushtofile

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

const secretGroupPrefix = "conjur.org/conjur-secrets."
const secretGroupPolicyPathPrefix = "conjur.org/conjur-secrets-policy-path."
const secretGroupFileTemplatePrefix = "conjur.org/secret-file-template."
const secretGroupFilePathPrefix = "conjur.org/secret-file-path."
const secretGroupFileFormatPrefix = "conjur.org/secret-file-format."

const defaultFilePermissions os.FileMode = 0664

type SecretGroup struct {
	Name             string
	FilePath         string
	FileTemplate     string
	FileFormat       string
	PolicyPathPrefix string
	FilePermissions  os.FileMode
	SecretSpecs      []SecretSpec
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
func NewSecretGroups(secretsBasePath string, annotations map[string]string) ([]*SecretGroup, []error) {
	var sgs []*SecretGroup

	var errors []error
	for k, v := range annotations {
		if strings.HasPrefix(k, secretGroupPrefix) {
			groupName := strings.TrimPrefix(k, secretGroupPrefix)
			secretSpecs, err := NewSecretSpecs([]byte(v))
			if err != nil {
				// Accumulate errors
				err = fmt.Errorf(
					`cannot create secret specs from annotation "%s": %s`,
					k,
					err,
				)
				errors = append(errors, err)
				continue
			}

			fileTemplate := annotations[secretGroupFileTemplatePrefix+groupName]
			filePath := annotations[secretGroupFilePathPrefix+groupName]
			fileFormat := annotations[secretGroupFileFormatPrefix+groupName]
			policyPathPrefix := annotations[secretGroupPolicyPathPrefix+groupName]

			if len(fileFormat) > 0 {
				_, err := FileTemplateForFormat(fileFormat, secretSpecs)
				if err != nil {
					// Accumulate errors
					err = fmt.Errorf(
						`unable to process file format annotation %q for group: %s`,
						fileFormat,
						err,
					)
					errors = append(errors, err)
					continue
				}
			}

			if filePath[0] == '/' {
				errors = append(errors, fmt.Errorf(
					"provided filepath '%s' for secret group '%s' is absolute: requires relative path",
					filePath, groupName,
				))
				continue
			}
			// Create filepath from secrets base path, provided filepath, and group name
			// Join the provided filepath to the static base path
			// If the filepath points to a directory, use the group name as the file name
			// If the group has a configured file template, filepath must point to a file
			fullPath := path.Join(secretsBasePath, filePath)
			if filePath[len(filePath)-1:] == "/" {
				if len(fileTemplate) > 0 {
					errors = append(errors, fmt.Errorf(
						"provided filepath '%s' for secret group '%s' must specify a file: required when 'conjur.org/secret-file-template.%s' is configured",
						filePath, groupName, groupName,
					))
				} else {
					fullPath = path.Join(fullPath, fmt.Sprintf("%s.%s", groupName, fileFormat))
				}
			}

			sgs = append(sgs, &SecretGroup{
				Name:             groupName,
				FilePath:         fullPath,
				FileTemplate:     fileTemplate,
				FileFormat:       fileFormat,
				FilePermissions:  defaultFilePermissions,
				PolicyPathPrefix: policyPathPrefix,
				SecretSpecs:      secretSpecs,
			})
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
