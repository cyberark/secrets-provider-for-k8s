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
	secretsConfigProvider "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
)

// log
var errorLogger = log.ErrorLogger
var infoLogger = log.InfoLogger

func main() {
	var err error

	// Initialize configurations
	authnConfig, err := authnConfigProvider.NewFromEnv()
	if err != nil {
		printErrorAndExit(messages.CSPFK008E)
	}

	secretsConfig, err := secretsConfigProvider.NewFromEnv()
	if err != nil {
		printErrorAndExit(messages.CSPFK015E)
	}

	accessToken, err := memory.NewAccessToken()
	if err != nil {
		printErrorAndExit(messages.CSPFK001E)
	}

	pushConjurSecrets, err := k8s_secrets_storage.NewProvideConjurSecrets(*secretsConfig, accessToken)
	if err != nil {
		printErrorAndExit(messages.CSPFK014E)
	}

	if secretsConfig.StoreType == secretsConfigProvider.K8S && authnConfig.ContainerMode != "init" {
		printErrorAndExit(messages.CSPFK007E)
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
			infoLogger.Printf(fmt.Sprintf(messages.CSPFK102I, authn.Config.Username))
			authnResp, err := authn.Authenticate()
			if err != nil {
				return log.RecorderError(messages.CSPFK010E)
			}

			err = authn.ParseAuthenticationResponse(authnResp)
			if err != nil {
				return log.RecorderError(messages.CSPFK011E)
			}

			err = pushConjurSecrets.Run()
			if err != nil {
				return log.RecorderError(messages.CSPFK016E)
			}

			err = accessToken.Delete()
			if err != nil {
				return log.RecorderError(messages.CSPFK003E, err.Error())
			}

			if authnConfig.ContainerMode == "init" {
				os.Exit(0)
			}

			// Reset exponential backoff
			expBackoff.Reset()

			infoLogger.Printf(messages.CSPFK108I, authn.Config.TokenRefreshTimeout)

			fmt.Println()
			time.Sleep(authn.Config.TokenRefreshTimeout)
		}
	}, expBackoff)

	if err != nil {
		// Deleting the retrieved Conjur access token in case we got an error after retrieval.
		// if the access token is already deleted the action should not fail
		err = accessToken.Delete()
		if err != nil {
			errorLogger.Printf(messages.CSPFK003E, err)
		}

		printErrorAndExit(messages.CSPFK038E)
	}
}

func printErrorAndExit(errorMessage string) {
	errorLogger.Printf(errorMessage)
	os.Exit(1)
}
