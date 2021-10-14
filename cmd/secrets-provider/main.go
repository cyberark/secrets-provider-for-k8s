package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
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

	// Initialize Secrets ProviderFunc configuration
	secretsConfig := secretsConfigProvider.NewConfig(secretsProviderSettings)

	// Select ProviderFunc
	provideSecrets, err := secrets.NewProviderForType(
		secretsConfig.StoreType,
		secretsProviderSettings,
	)
	if err != nil {
		printErrorAndExit(fmt.Sprintf(messages.CSPFK014E, err.Error()))
	}

	accessToken, err := memory.NewAccessToken()
	// Always delete access token. The deletion idempotent and never fails
	defer accessToken.Delete()
	if err != nil {
		printErrorAndExit(messages.CSPFK001E)
	}

	authn, err := authenticator.NewWithAccessToken(*authnConfig, accessToken)
	if err != nil {
		printErrorAndExit(messages.CSPFK009E)
	}

	// TODO^: construction of authenticator can take place in an independent method
	secretFetcher := conjur.NewConjurSecretFetcher(authn)

	// Make provider retriable
	provideSecrets = secrets.RetriableSecretProvider(
		time.Duration(secretsConfig.RetryIntervalSec)*time.Second,
		secretsConfig.RetryCountLimit,
		provideSecrets,
	)

	err = provideSecrets(secretFetcher.Fetch)
	if err != nil {
		printErrorAndExit(messages.CSPFK039E)
	}
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
