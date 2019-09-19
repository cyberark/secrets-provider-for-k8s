package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator"
	authnConfigProvider "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets"
	secretsConfigProvider "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/config"
)

func main() {
	var err error

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

	// verify we don't run with a K8s secrets storage configuration in a side car
	if secretsConfig.StoreType == secretsConfigProvider.K8S && authnConfig.ContainerMode != "init" {
		printErrorAndExit(messages.CSPFK007E)
	}

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

	// Configure exponential backoff
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = 2 * time.Second
	expBackoff.RandomizationFactor = 0.5
	expBackoff.Multiplier = 2
	expBackoff.MaxInterval = 15 * time.Second
	expBackoff.MaxElapsedTime = 2 * time.Minute

	err = backoff.Retry(func() error {
		for {
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

			if authnConfig.ContainerMode == "init" {
				os.Exit(0)
			}

			// Reset exponential backoff
			expBackoff.Reset()

			log.Info(messages.CSPFK007I, authn.Config.TokenRefreshTimeout)

			fmt.Println()
			time.Sleep(authn.Config.TokenRefreshTimeout)
		}
	}, expBackoff)

	if err != nil {
		// Deleting the retrieved Conjur access token in case we got an error after retrieval.
		// if the access token is already deleted the action should not fail
		err = accessToken.Delete()
		if err != nil {
			log.Error(messages.CSPFK003E, err)
		}

		printErrorAndExit(messages.CSPFK038E)
	}
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
