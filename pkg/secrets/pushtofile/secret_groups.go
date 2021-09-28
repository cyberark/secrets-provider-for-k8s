package pushtofile

import (
	"os"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

const (
	conjurSecretsPrefix           = "conjur.org/conjur-secrets."
	conjurSecretsPolicyPathPrefix = "conjur.org/conjur-secrets-policy-path."
	secretFilePathPrefix          = "conjur.org/secret-file-path."
	secretFileFormatPrefix        = "conjur.org/secret-file-format."
	secretFileTemplatePrefix      = "conjur.org/secret-file-template."

	defaultFilePerms os.FileMode = 0777
)

// SecretGroup comprises secrets mapping information for a given secrets
// group.
type SecretGroup struct {
	Label                  string
	FilePath               string
	FileTemplate           string
	ConjurSecretPathPrefix string
	SecretSpecs            []SecretSpec
	FileFormat             string
	FilePerms              os.FileMode
}

// SecretGroups comprises secrets mapping info for all secret groups
type SecretGroups []SecretGroup

func NewSecretGroupsFromAnnotations(annotations map[string]string) (SecretGroups, error) {
	secretsGroups := []SecretGroup{}

	for annotation := range annotations {
		if strings.HasPrefix(annotation, conjurSecretsPrefix) {
			groupName := strings.TrimPrefix(annotation, conjurSecretsPrefix)
			conjurSecretPathPrefix := annotations[conjurSecretsPolicyPathPrefix+groupName]
			filePath := annotations[secretFilePathPrefix+groupName]
			fileTemplate := annotations[secretFileTemplatePrefix+groupName]

			fileFormat, err := parseFileFormat(annotations[secretFileFormatPrefix+groupName])
			if err != nil {
				return nil, err
			}

			secrets, err := NewSecretSpecs([]byte(annotations[conjurSecretsPrefix+groupName]))
			if err != nil {
				return nil, err
			}

			group := SecretGroup{
				Label:                  groupName,
				FilePath:               filePath,
				FileTemplate:           fileTemplate,
				ConjurSecretPathPrefix: conjurSecretPathPrefix,
				SecretSpecs:            secrets,
				FileFormat:             fileFormat,
				FilePerms:              defaultFilePerms,
			}

			secretsGroups = append(secretsGroups, group)
		}
	}

	return secretsGroups, nil
}

func parseFileFormat(fileFormat string) (string, error) {
	switch fileFormat {
	case "yaml":
		fallthrough
	case "json":
		fallthrough
	case "dotenv":
		fallthrough
	case "bash":
		fallthrough
	case "plaintext":
		return fileFormat, nil
	case "":
		return "yaml", nil
	default:
		return "", log.RecordedError(messages.CSPFK052E, fileFormat)
	}
}
