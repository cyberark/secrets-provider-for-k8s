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
	secretsConfigProvider "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

func main() {
	var err error

	log.Info(messages.CSPFK008I, secrets.FullVersionName)

	configureLogLevel()

	// Initialize configurations
	authnConfig, err := authnConfigProvider.NewFromEnv()
	if err != nil {
		printErrorAndExit(messages.CSPFK008E)
	}

	secretsConfig, err := secretsConfigProvider.NewFromEnv()
	if err != nil {
		printErrorAndExit(messages.CSPFK015E)
	}

	validateContainerMode(authnConfig.ContainerMode)

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

		return provideSecretsToTarget(authn, provideConjurSecrets, accessToken)
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

func provideSecretsToTarget(authn *authenticator.Authenticator, provideConjurSecrets secrets.ProvideConjurSecrets, accessToken *memory.AccessToken) error {
	log.Info(fmt.Sprintf(messages.CSPFK001I, authn.Config.Username))
	authnResp, err := authn.Authenticate()
	if err != nil {
		return log.RecordedError(messages.CSPFK010E)
	}

	err = authn.ParseAuthenticationResponse(authnResp)
	if err != nil {
		return log.RecordedError(messages.CSPFK011E)
	}

	err = provideConjurSecrets(accessToken)
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

func configureLogLevel() {
	validVal := "true"
	val := os.Getenv("DEBUG")
	if val == validVal {
		log.EnableDebugMode()
	} else if val != "" {
		// In case "DEBUG" is configured with incorrect value
		log.Warn(messages.CSPFK001W, val, validVal)
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
