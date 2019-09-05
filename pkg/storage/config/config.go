package config

import (
	"os"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
)

type Config struct {
	StoreType string
}

const (
	K8S                      = "k8s_secrets"
	SecretsDestinationEnvVar = "SECRETS_DESTINATION"
)

func NewFromEnv() (*Config, error) {
	var storeType string

	secretsDestinationValue := os.Getenv(SecretsDestinationEnvVar)
	if secretsDestinationValue == K8S {
		storeType = K8S
	} else if secretsDestinationValue == "" {
		// TODO: decide what to do in this case
		storeType = K8S
	} else {
		// In case SecretsDestinationEnvVar exists and is configured with incorrect value
		return nil, log.RecorderError(messages.CSPFK005E, SecretsDestinationEnvVar)
	}
	return &Config{
		StoreType: storeType,
	}, nil
}
