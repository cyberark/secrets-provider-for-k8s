package main

import (
	"fmt"
	"os"
	"time"

	authnConfigProvider "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	secretsConfigProvider "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
)

const annotationsFile = "/conjur/podinfo/annotations"

func main() {
	var err error

	log.Info(messages.CSPFK008I, secrets.FullVersionName)

	// Initialize authn configuration
	authnConfig, err := authnConfigProvider.NewFromEnv()
	if err != nil {
		printErrorAndExit(messages.CSPFK008E)
	}

	// Parse annotations
	annotationsMap := map[string]string{}
	// Only attempt to populate from annotations if the annotations file exists
	if _, err := os.Stat(annotationsFile); err == nil {
		annotationsMap, err = annotations.NewAnnotationsFromFile(annotationsFile)
		if err != nil {
			printErrorAndExit(messages.CSPFK040E)
		}
	}

	errLogs, infoLogs := secretsConfigProvider.ValidateAnnotations(annotationsMap)
	logErrorsAndConditionalExit(errLogs, infoLogs, messages.CSPFK049E)

	secretsProviderSettings := secretsConfigProvider.GatherSecretsProviderSettings(annotationsMap)

	errLogs, infoLogs = secretsConfigProvider.ValidateSecretsProviderSettings(secretsProviderSettings)
	logErrorsAndConditionalExit(errLogs, infoLogs, messages.CSPFK015E)

	// Initialize Secrets Provider configuration
	secretsConfig := secretsConfigProvider.NewConfig(secretsProviderSettings)

	// Select Provider
	provideSecrets, err := secrets.NewProviderForType(
		secretsConfig.StoreType,
		secretsProviderSettings,
	)
	if err != nil {
		printErrorAndExit(fmt.Sprintf(messages.CSPFK014E, err.Error()))
	}

	// Make Provider retriable
	provideSecrets = secrets.RetriableSecretProvider(
		time.Duration(secretsConfig.RetryIntervalSec)*time.Second,
		secretsConfig.RetryCountLimit,
		provideSecrets,
	)

	// Create Conjur secret fetcher
	var fetchSecrets conjur.FetchSecretsFunc
	secretFetcher, err := conjur.NewConjurSecretFetcher(*authnConfig)
	if err != nil {
		printErrorAndExit(err.Error())
	}
	fetchSecrets = secretFetcher.Fetch

	// Provide secrets
	err = provideSecrets(fetchSecrets)
	if err != nil {
		printErrorAndExit(fmt.Sprintf(messages.CSPFK039E, secretsConfig.StoreType))
	}
	log.Info(fmt.Sprintf(messages.CSPFK009I, secretsConfig.StoreType))
}

func printErrorAndExit(errorMessage string) {
	log.Error(errorMessage)
	os.Exit(1)
}

func logErrorsAndConditionalExit(errLogs []error, infoLogs []error, failureMsg string) {
	for _, err := range infoLogs {
		log.Info(err.Error())
	}
	if len(errLogs) > 0 {
		for _, err := range errLogs {
			log.Error(err.Error())
		}
		printErrorAndExit(failureMsg)
	}
}
