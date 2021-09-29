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
	if len(secrets) == 0 {
		return fmt.Errorf("%s", "list of resolved secrets on group is empty")
	}

	if len(s.FileTemplate)+len(s.FileFormat) == 0 {
		return fmt.Errorf("%s", `missing one of "file template" or "file format" for group`)
	}

	fileTemplate := s.FileTemplate

	if len(fileTemplate) == 0 && len(s.FileFormat) > 0 {
		stdTemplate, ok := standardTemplates[s.FileFormat]
		if !ok {
			return fmt.Errorf(`unrecognized file format provided, "%s"`, s.FileFormat)
		}

		for _, s := range secrets {
			err := stdTemplate.ValidateAlias(s.Alias)
			if err != nil {
				return err
			}
		}

		fileTemplate = stdTemplate.Template
	}

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

// NewSecretGroups creates a collection of secret groups from a map of annotations
func NewSecretGroups(annotations map[string]string) ([]*SecretGroup, []error) {
	var sgs []*SecretGroup

	var errors []error
	for k, v := range annotations {
		if strings.HasPrefix(k, secretGroupPrefix) {
			groupName := strings.TrimPrefix(k, secretGroupPrefix)
			secretSpecs, err := NewSecretSpecs([]byte(v))
			if err != nil {
				// Accumulate errors
				errors = append(errors, err)
				continue
			}

			fileTemplate := annotations[secretGroupFileTemplatePrefix+groupName]
			filePath := annotations[secretGroupFilePathPrefix+groupName]
			fileFormat := annotations[secretGroupFileFormatPrefix+groupName]
			policyPathPrefix := annotations[secretGroupPolicyPathPrefix+groupName]

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

	if len(errors) > 0 {
		return nil, errors
	}

	// Sort secret groups for deterministic order based on group path
	sort.SliceStable(sgs, func(i, j int) bool {
		return sgs[i].Name < sgs[j].Name
	})

	return sgs, nil
}
