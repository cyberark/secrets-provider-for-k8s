package config

import (
	"os"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
)

type Config struct {
	StoreType     string
	TokenFilePath string
}

const (
	K8S                  = "k8s_secrets"
	None                 = "none"
	SecretsDestination   = "SECRETS_DESTINATION"
	TokenFilePathDefault = "/run/conjur/access-token"
)

func NewFromEnv() (*Config, error) {
	storeType := None
	tokenFilePath := TokenFilePathDefault
	secretsDestinationValue := os.Getenv(SecretsDestination)
	if secretsDestinationValue == K8S {
		storeType = K8S
		tokenFilePath = ""
	} else if secretsDestinationValue == "" || secretsDestinationValue == None {
		storeType = None
		// If CONJUR_TOKEN_FILE_PATH not configured take default value
		if envVal := os.Getenv("CONJUR_AUTHN_TOKEN_FILE"); envVal != "" {
			tokenFilePath = envVal
		}
	} else {
		// In case SecretsDestination exits and has configured with incorrect value
		return nil, log.RecorderError(log.CSPFK042E, SecretsDestination)
	}
	return &Config{
		StoreType:     storeType,
		TokenFilePath: tokenFilePath,
	}, nil
}
