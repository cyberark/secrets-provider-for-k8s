package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	authnConfigProvider "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	secretsConfigProvider "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
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

	validateContainerMode(authnConfig.ContainerMode)

	annotationsMap := map[string]string{}
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

	provideConjurSecrets, err := secrets.GetProvideConjurSecretFunc(secretsConfig.StoreType)
	if err != nil {
		printErrorAndExit(fmt.Sprintf(messages.CSPFK014E, err.Error()))
	}

	accessToken, err := memory.NewAccessToken()
	if err != nil {
		printErrorAndExit(messages.CSPFK001E)
	}

	authn, err := authenticator.NewWithAccessToken(*authnConfig, accessToken)
	if err != nil {
		printErrorAndExit(messages.CSPFK009E)
	}

	limitedBackOff := utils.NewLimitedBackOff(
		time.Duration(secretsConfig.RetryIntervalSec)*time.Second,
		secretsConfig.RetryCountLimit)

	err = backoff.Retry(func() error {
		if limitedBackOff.RetryCount() > 0 {
			log.Info(fmt.Sprintf(messages.CSPFK010I, limitedBackOff.RetryCount(), limitedBackOff.RetryLimit))
		}

		return provideSecretsToTarget(authn, provideConjurSecrets, accessToken, secretsConfig)
	}, limitedBackOff)

	if err != nil {
		log.Error(messages.CSPFK038E)

		// Deleting the retrieved Conjur access token in case we got an error after retrieval.
		// if the access token is already deleted the action should not fail
		err = accessToken.Delete()
		if err != nil {
			log.Error(messages.CSPFK003E, err)
		}
		printErrorAndExit(messages.CSPFK039E)
	}
}

func provideSecretsToTarget(authn *authenticator.Authenticator, provideConjurSecrets secrets.ProvideConjurSecrets,
	accessToken *memory.AccessToken, secretsConfig *secretsConfigProvider.Config) error {
	log.Info(fmt.Sprintf(messages.CSPFK001I, authn.Config.Username))
	err := authn.Authenticate()
	if err != nil {
		return log.RecordedError(messages.CSPFK010E)
	}

	err = provideConjurSecrets(accessToken, secretsConfig)
	if err != nil {
		return log.RecordedError(messages.CSPFK016E)
	}

	err = accessToken.Delete()
	if err != nil {
		return log.RecordedError(messages.CSPFK003E, err.Error())
	}

	log.Info(messages.CSPFK009I)
	return nil
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

func validateContainerMode(containerMode string) {
	validContainerModes := []string{
		"init",
		"application",
	}

	isValidContainerMode := false
	for _, validContainerModeType := range validContainerModes {
		if containerMode == validContainerModeType {
			isValidContainerMode = true
		}
	}

	if !isValidContainerMode {
		printErrorAndExit(fmt.Sprintf(messages.CSPFK007E, containerMode, validContainerModes))
	}
}
