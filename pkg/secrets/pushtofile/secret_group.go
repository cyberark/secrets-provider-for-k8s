package pushtofile

import (
	"fmt"
	"io/ioutil"
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
const maxFilenameLen = 255

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
// Updates the Path of each SecretSpec in the field SecretSpecs.
func resolvedSecretSpecs(policyPathPrefix string, secretSpecs []SecretSpec) []SecretSpec {
	if len(policyPathPrefix) != 0 {
		for i := range secretSpecs {
			secretSpecs[i].Path = strings.TrimSuffix(policyPathPrefix, "/") +
				"/" + strings.TrimPrefix(secretSpecs[i].Path, "/")
		}
	}

	return secretSpecs
}

// PushToFile uses the configuration on a secret group to inject secrets into a template
// and write the result to a file.
func (sg *SecretGroup) PushToFile(secrets []*Secret) error {
	return sg.pushToFileWithDeps(openFileAsWriteCloser, pushToWriter, secrets)
}

func (sg *SecretGroup) pushToFileWithDeps(
	depOpenWriteCloser openWriteCloserFunc,
	depPushToWriter pushToWriterFunc,
	secrets []*Secret,
) (err error) {
	// Make sure all the secret specs are accounted for
	err = validateSecretsAgainstSpecs(secrets, sg.SecretSpecs)
	if err != nil {
		return
	}

	// Determine file template from
	// 1. File template
	// 2. File format
	// 3. Secret specs (user to validate file template)
	fileTemplate, err := maybeFileTemplateFromFormat(
		sg.FileTemplate,
		sg.FileFormat,
		sg.SecretSpecs,
	)
	if err != nil {
		return
	}

	//// Open and push to file
	wc, err := depOpenWriteCloser(sg.FilePath, sg.FilePermissions)
	if err != nil {
		return
	}
	defer func() {
		_ = wc.Close()
	}()

	maskError := fmt.Errorf("failed to execute template, with secret values, on push to file for secret group %q", sg.Name)
	defer func() {
		if r := recover(); r != nil {
			err = maskError
		}
	}()
	pushToWriterErr := depPushToWriter(
		wc,
		sg.Name,
		fileTemplate,
		secrets,
	)
	if pushToWriterErr != nil {
		err = maskError
	}
	return
}

func (sg *SecretGroup) absoluteFilePath(secretsBasePath string) (string, error) {
	groupName := sg.Name
	filePath := sg.FilePath
	fileTemplate := sg.FileTemplate
	fileFormat := sg.FileFormat

	// filePath must be relative
	if path.IsAbs(filePath) {
		return "", fmt.Errorf(
			"provided filepath %q for secret group %q is absolute, requires relative path",
			filePath, groupName,
		)
	}

	pathContainsFilename := !strings.HasSuffix(filePath, "/") && len(filePath) > 0

	if !pathContainsFilename {
		if len(fileTemplate) > 0 {
			// fileTemplate requires filePath to point to a file (not a directory)
			return "", fmt.Errorf(
				"provided filepath %q for secret group %q must specify a path to a file, without a trailing %q",
				filePath, groupName, "/",
			)
		}

		// Without the restrictions of fileTemplate, the filename defaults to "{groupName}.{fileFormat}"
		filePath = path.Join(
			filePath,
			fmt.Sprintf("%s.%s", groupName, fileFormat),
		)
	}

	absoluteFilePath := path.Join(secretsBasePath, filePath)

	// filePath must be relative to secrets base path. This protects against relative paths
	// that, by using the double-dot path segment, resolve to a path that is not relative
	// to the base path.
	if !strings.HasPrefix(absoluteFilePath, secretsBasePath) {
		return "", fmt.Errorf(
			"provided filepath %q for secret group %q must be relative to secrets base path",
			filePath, groupName,
		)
	}

	// Filename cannot be longer than allowed by the filesystem
	_, filename := path.Split(absoluteFilePath)
	if len(filename) > maxFilenameLen {
		return "", fmt.Errorf(
			"filename %q for provided filepath for secret group %q must not be longer than %d characters",
			filename,
			groupName,
			maxFilenameLen,
		)
	}

	return absoluteFilePath, nil
}

func (sg *SecretGroup) validate() []error {
	groupName := sg.Name
	fileFormat := sg.FileFormat
	fileTemplate := sg.FileTemplate
	secretSpecs := sg.SecretSpecs

	if errors := validateSecretPaths(secretSpecs, groupName); len(errors) > 0 {
		return errors
	}

	if len(fileFormat) > 0 {
		_, err := FileTemplateForFormat(fileFormat, secretSpecs)
		if err != nil {
			return []error{
				fmt.Errorf(
					"unable to process file format %q for group: %s",
					fileFormat,
					err,
				),
			}
		}
	}

	// First-pass at provided template rendering with dummy secret values
	// This first-pass is limited for templates that branch conditionally on secret values
	// Relying logically on specific secret values should be avoided
	if len(fileTemplate) > 0 {
		dummySecrets := []*Secret{}
		for _, secretSpec := range secretSpecs {
			dummySecrets = append(dummySecrets, &Secret{Alias: secretSpec.Alias, Value: "REDACTED"})
		}

		err := pushToWriter(ioutil.Discard, groupName, fileTemplate, dummySecrets)
		if err != nil {
			return []error{fmt.Errorf(
				`unable to use file template for secret group %q: %s`,
				groupName,
				err,
			)}
		}
	}

	return nil
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
	// Default to "yaml" file format
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
	for key := range annotations {
		if strings.HasPrefix(key, secretGroupPrefix) {
			groupName := strings.TrimPrefix(key, secretGroupPrefix)
			sg, errs := newSecretGroup(groupName, secretsBasePath, annotations)
			if errs != nil {
				errors = append(errors, errs...)
				continue
			}
			sgs = append(sgs, sg)
		}
	}

	errors = append(errors, validateGroupFilePaths(sgs)...)

	if len(errors) > 0 {
		return nil, errors
	}

	// Sort secret groups for deterministic order based on group path
	sort.SliceStable(sgs, func(i, j int) bool {
		return sgs[i].Name < sgs[j].Name
	})

	return sgs, nil
}

func newSecretGroup(groupName string, secretsBasePath string, annotations map[string]string) (*SecretGroup, []error) {
	groupSecrets := annotations[secretGroupPrefix+groupName]
	fileTemplate := annotations[secretGroupFileTemplatePrefix+groupName]
	filePath := annotations[secretGroupFilePathPrefix+groupName]
	fileFormat := annotations[secretGroupFileFormatPrefix+groupName]
	policyPathPrefix := annotations[secretGroupPolicyPathPrefix+groupName]
	policyPathPrefix = strings.TrimPrefix(policyPathPrefix, "/")

	// Default to "yaml" file format
	if len(fileTemplate)+len(fileFormat) == 0 {
		fileFormat = "yaml"
	}

	secretSpecs, err := NewSecretSpecs([]byte(groupSecrets))
	if err != nil {
		err = fmt.Errorf(`unable to create secret specs from annotation "%s": %s`, secretGroupPrefix+groupName, err)
		return nil, []error{err}
	}
	secretSpecs = resolvedSecretSpecs(policyPathPrefix, secretSpecs)

	sg := &SecretGroup{
		Name:             groupName,
		FilePath:         filePath,
		FileTemplate:     fileTemplate,
		FileFormat:       fileFormat,
		FilePermissions:  defaultFilePermissions,
		PolicyPathPrefix: policyPathPrefix,
		SecretSpecs:      secretSpecs,
	}

	errors := sg.validate()
	if len(errors) > 0 {
		return nil, errors
	}

	// Generate absolute file path
	sg.FilePath, err = sg.absoluteFilePath(secretsBasePath)
	if err != nil {
		return nil, []error{err}
	}

	return sg, nil
}

func validateGroupFilePaths(secretGroups []*SecretGroup) []error {
	// Iterate over the secret groups and group any that have the same file path
	groupFilePaths := make(map[string][]string)
	for _, sg := range secretGroups {
		if len(groupFilePaths[sg.FilePath]) == 0 {
			groupFilePaths[sg.FilePath] = []string{sg.Name}
			continue
		}

		groupFilePaths[sg.FilePath] = append(groupFilePaths[sg.FilePath], sg.Name)
	}

	// If any file paths are used in more than one group, log all the groups that share the path
	var errors []error
	for path, groupNames := range groupFilePaths {
		if len(groupNames) > 1 {
			errors = append(errors, fmt.Errorf(
				"duplicate filepath %q for groups: %q", path, strings.Join(groupNames, `, `),
			))
		}
	}
	return errors
}
