package pushtofile

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"gopkg.in/yaml.v3"
)

const (
	CONJUR_SECRETS_PREFIX             = "conjur.org/conjur-secrets."
	CONJUR_SECRETS_POLICY_PATH_PREFIX = "conjur.org/conjur-secrets-policy-path."
	SECRET_FILE_PATH_PREFIX           = "conjur.org/secret-file-path."
	SECRET_FILE_FORMAT_PREFIX         = "conjur.org/secret-file-format."
	SECRET_FILE_TEMPLATE_PREFIX       = "conjur.org/secret-file-template."

	DEFAULT_FILE_PERMS os.FileMode = 0777
	DEFAULT_FILE_PATH  string      = "/conjur/secrets/"
)

type SecretsFileFormat int

const (
	FILE_FORMAT_NONE SecretsFileFormat = iota
	FILE_FORMAT_YAML
	FILE_FORMAT_JSON
	FILE_FORMAT_DOTENV
	FILE_FORMAT_BASH
	FILE_FORMAT_PLAINTEXT
)

// SecretsPaths comprises Conjur variable paths for all secrets in a secrets
// group, indexed by secret name.
type SecretsPaths map[string]string

// SecretsGroupInfo comprises secrets mapping information for a given secrets
// group.
type SecretsGroupInfo struct {
	Secrets      SecretsPaths
	FilePath     string
	FileFormat   SecretsFileFormat
	FilePerms    os.FileMode
	FileTemplate string
}

// SecretsGroups comprises secrets mapping info for all secret groups
type SecretsGroups map[string]SecretsGroupInfo

func ExtractSecretsGroupsFromAnnotations(annotations map[string]string) (SecretsGroups, error) {
	secretsGroups := make(SecretsGroups)

	for annotation := range annotations {
		if strings.HasPrefix(annotation, CONJUR_SECRETS_PREFIX) {
			groupName := strings.TrimPrefix(annotation, CONJUR_SECRETS_PREFIX)

			secretsPathPrefix, err := parseConjurSecretsPathPrefix(groupName, annotations[CONJUR_SECRETS_POLICY_PATH_PREFIX+groupName])
			if err != nil {
				return nil, err
			}

			secrets, err := parseConjurSecretsPaths(groupName, annotations[CONJUR_SECRETS_PREFIX+groupName], secretsPathPrefix)
			if err != nil {
				return nil, err
			}

			filePath, err := parseFilePath(groupName, annotations[SECRET_FILE_PATH_PREFIX+groupName])
			if err != nil {
				return nil, err
			}

			fileFormat, err := parseFileFormat(groupName, annotations[SECRET_FILE_FORMAT_PREFIX+groupName])
			if err != nil {
				return nil, err
			}

			fileTemplate := annotations[SECRET_FILE_TEMPLATE_PREFIX+groupName]

			if fileTemplate != "" {
				fileFormat = FILE_FORMAT_NONE
			}

			groupInfo := SecretsGroupInfo{
				Secrets:      secrets,
				FilePath:     filePath,
				FileFormat:   fileFormat,
				FilePerms:    DEFAULT_FILE_PERMS,
				FileTemplate: fileTemplate,
			}

			err = validateGroupInfo(groupName, groupInfo)
			if err != nil {
				return nil, err
			}

			secretsGroups[groupName] = groupInfo
		}
	}

	return secretsGroups, nil
}

func parseConjurSecretsPaths(groupName string, secretsPaths string, secretsPathsPrefix string) (SecretsPaths, error) {
	secrets := make(SecretsPaths)

	unpacked := []interface{}{}
	if err := yaml.Unmarshal([]byte(secretsPaths), &unpacked); err != nil {
		return nil, log.RecordedError(messages.CSPFK051E, err.Error(), groupName)
	}

	insertSecret := func(name string, secretPath string) error {
		secretPath = path.Clean(secretPath)
		if secretPath == "." {
			return log.RecordedError(messages.CSPFK051E, "Invalid secret path", groupName)
		}
		secrets[name] = secretsPathsPrefix + secretPath

		return nil
	}

	for _, secret := range unpacked {
		switch val := secret.(type) {
		case string:
			{
				name := path.Base(val)
				if name == "" {
					return nil, log.RecordedError(messages.CSPFK051E, "Invalid secret name", groupName)
				}

				if err := insertSecret(name, val); err != nil {
					return nil, err
				}
			}

		case map[string]interface{}:
			{
				for alias, secretPath := range val {
					secretPath, _ := secretPath.(string)
					if err := insertSecret(alias, secretPath); err != nil {
						return nil, err
					}
				}
			}

		default:
			return nil, log.RecordedError(messages.CSPFK051E, "Unknown secret format", groupName)
		}
	}

	if len(secrets) == 0 {
		return nil, log.RecordedError(messages.CSPFK051E, "No valid secrets found", groupName)
	}

	return secrets, nil
}

func parseConjurSecretsPathPrefix(groupName string, secretsPathPrefix string) (string, error) {
	// By default returns the root policy path '/'
	secretsPathPrefix = path.Clean("/" + secretsPathPrefix)
	if secretsPathPrefix == "." || secretsPathPrefix == "/" {
		return "/", nil
	}

	// Ensure policy path ends with '/'
	return secretsPathPrefix + "/", nil
}

func parseFilePath(groupName string, filePath string) (string, error) {
	// File path must be relative and can not contain a leading '/'
	if strings.HasPrefix(filePath, "/") {
		return "", log.RecordedError(messages.CSPFK052E, fmt.Sprintf("%s%s", SECRET_FILE_PATH_PREFIX, groupName))
	}

	// File path can be a directory (ending in /) or a file name (not ending in /)
	if strings.HasSuffix(filePath, "/") {
		filePath = path.Clean(filePath) + "/"
	} else {
		filePath = path.Clean(filePath)
	}

	if filePath == "." {
		return DEFAULT_FILE_PATH, nil
	}

	return DEFAULT_FILE_PATH + filePath, nil
}

func parseFileFormat(groupName string, fileFormat string) (SecretsFileFormat, error) {
	switch strings.ToLower(fileFormat) {
	case "":
		return FILE_FORMAT_YAML, nil
	case "yaml":
		return FILE_FORMAT_YAML, nil
	case "json":
		return FILE_FORMAT_JSON, nil
	case "dotenv":
		return FILE_FORMAT_DOTENV, nil
	case "bash":
		return FILE_FORMAT_BASH, nil
	case "plaintext":
		return FILE_FORMAT_PLAINTEXT, nil
	default:
		return FILE_FORMAT_NONE, log.RecordedError(messages.CSPFK053E, fmt.Sprintf("%s%s", SECRET_FILE_FORMAT_PREFIX, groupName), fileFormat)
	}
}

func validateGroupInfo(groupName string, groupInfo SecretsGroupInfo) error {
	// If a template is specified, then a secrets
	// file name is required, not just a directory
	if groupInfo.FileTemplate != "" {
		if strings.HasSuffix(groupInfo.FilePath, "/") {
			return log.RecordedError(messages.CSPFK054E, fmt.Sprintf("%s%s", SECRET_FILE_PATH_PREFIX, groupName))
		}
	}

	return nil
}
