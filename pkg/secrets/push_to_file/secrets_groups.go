package pushtofile

import (
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"gopkg.in/yaml.v3"
)

const SECRETS_GROUP_PREFIX = "conjur.org/conjur-secrets."
const SECRETS_GROUP_POLICY_PATH_PREFIX = "conjur.org/conjur-secrets-policy-path."
const SECRETS_GROUP_FILE_PATH_PREFIX = "conjur.org/secret-file-path."
const SECRETS_GROUP_FILE_TYPE_PREFIX = "conjur.org/secret-file-format."
const SECRETS_GROUP_FILE_TEMPLATE_PREFIX = "conjur.org/secret-file-template."

const DEFAULT_FILE_PERMS os.FileMode = 0777
const DEFAULT_FILE_PATH string = "/conjur/secrets/"

type SecretsFileType int

const (
	FILE_TYPE_NONE SecretsFileType = iota
	FILE_TYPE_YAML
	FILE_TYPE_JSON
	FILE_TYPE_DOTENV
	FILE_TYPE_BASH
	FILE_TYPE_PLAINTEXT
)

// SecretsPaths comprises Conjur variable paths for all secrets in a secrets
// group, indexed by secret name.
type SecretsPaths map[string]string

// SecretsGroupInfo comprises secrets mapping information for a given secrets
// group.
type SecretsGroupInfo struct {
	Secrets      SecretsPaths
	FilePath     string
	FileType     SecretsFileType
	FilePerms    os.FileMode
	FileTemplate *template.Template
}

// SecretsGroups comprises secrets mapping info for all secret groups
type SecretsGroups map[string]SecretsGroupInfo

func ExtractSecretsGroupsFromAnnotations(annotations map[string]string) (SecretsGroups, error) {
	secretsGroups := make(SecretsGroups)

	for annotation := range annotations {
		if strings.HasPrefix(annotation, SECRETS_GROUP_PREFIX) {
			groupName := strings.TrimPrefix(annotation, SECRETS_GROUP_PREFIX)

			secretsPathPrefix, err := parseSecretsPathPrefix(groupName, annotations[SECRETS_GROUP_POLICY_PATH_PREFIX+groupName])
			if err != nil {
				return nil, err
			}

			secrets, err := parseSecretsPaths(groupName, annotations[SECRETS_GROUP_PREFIX+groupName], secretsPathPrefix)
			if err != nil {
				return nil, err
			}

			filePath, err := parseFilePath(groupName, annotations[SECRETS_GROUP_FILE_PATH_PREFIX+groupName])
			if err != nil {
				return nil, err
			}

			fileType, err := parseFileType(groupName, annotations[SECRETS_GROUP_FILE_TYPE_PREFIX+groupName])
			if err != nil {
				return nil, err
			}

			fileTemplate, err := parseFileTemplate(groupName, annotations[SECRETS_GROUP_FILE_TEMPLATE_PREFIX+groupName])
			if err != nil {
				return nil, err
			}

			if fileTemplate != nil {
				fileType = FILE_TYPE_NONE
			}

			configFileSrcPath, err := parseConfigFilePath(annotations, groupName, SECRETS_GROUP_CONFIG_FILE_SRC_PATH_PREFIX)
			if err != nil {
				return nil, err
			}

			configfileDestPath, err := parseConfigFilePath(annotations, groupName, SECRETS_GROUP_CONFIG_FILE_DEST_PATH_PREFIX)
			if err != nil {
				return nil, err
			}

			groupInfo := SecretsGroupInfo{
				Secrets:      secrets,
				FilePath:     filePath,
				FileType:     fileType,
				FilePerms:    DEFAULT_FILE_PERMS,
				FileTemplate: fileTemplate,
			}

			err = validateGroupInfo(groupName, groupInfo)
			if err != nil {
				return nil, err
			}

			/*
				if groupInfo.FileTemplate != nil {
					groupInfo.FileTemplate.Execute(os.Stdout, groupInfo.Secrets)
				}
			*/

			secretsGroups[groupName] = groupInfo
		}
	}

	return secretsGroups, nil
}

func parseSecretsPaths(groupName string, secretsPaths string, secretsPathsPrefix string) (SecretsPaths, error) {
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
				name := val[strings.LastIndex(val, "/")+1:]
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

func parseSecretsPathPrefix(groupName string, secretsPathPrefix string) (string, error) {
	secretsPathPrefix = path.Clean("/" + secretsPathPrefix)
	if secretsPathPrefix == "." || secretsPathPrefix == "/" {
		return "/", nil
	}

	return secretsPathPrefix + "/", nil
}

func parseFilePath(groupName string, filePath string) (string, error) {
	if strings.HasPrefix(filePath, "/") {
		return "", log.RecordedError(messages.CSPFK052E, fmt.Sprintf("%s%s", SECRETS_GROUP_FILE_PATH_PREFIX, groupName))
	}

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

func parseFileType(groupName string, fileType string) (SecretsFileType, error) {
	switch strings.ToLower(fileType) {
	case "":
		return FILE_TYPE_YAML, nil
	case "yaml":
		return FILE_TYPE_YAML, nil
	case "json":
		return FILE_TYPE_JSON, nil
	case "dotenv":
		return FILE_TYPE_DOTENV, nil
	case "bash":
		return FILE_TYPE_BASH, nil
	case "plaintext":
		return FILE_TYPE_PLAINTEXT, nil
	default:
		return FILE_TYPE_NONE, log.RecordedError(messages.CSPFK053E, fmt.Sprintf("%s%s", SECRETS_GROUP_FILE_TYPE_PREFIX, groupName), fileType)
	}
}

func parseFileTemplate(groupName string, fileTemplate string) (*template.Template, error) {
	if fileTemplate == "" {
		return nil, nil
	}

	t, err := template.New("FileTemplate").Parse(fileTemplate)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK054E, fmt.Sprintf("%s%s", SECRETS_GROUP_FILE_TEMPLATE_PREFIX, groupName), err.Error())
	}

	return t, nil
}

func parseConfigFilePath(annotations map[string]string, groupName, prefix string) error {
	annotationID := prefix + groupName
	filePath := annotations[annotationID]
	if strings.HasSuffix(filePath, "/") {
		return log.RecordedError(messages.CSPFK056E, fmt.Sprintf(annotationID))
	}
}

func validateGroupInfo(groupName string, groupInfo SecretsGroupInfo) error {
	if groupInfo.FileTemplate != nil {
		if strings.HasSuffix(groupInfo.FilePath, "/") {
			return log.RecordedError(messages.CSPFK055E, fmt.Sprintf("%s%s", SECRETS_GROUP_FILE_PATH_PREFIX, groupName))
		}
	}

	return nil
}
