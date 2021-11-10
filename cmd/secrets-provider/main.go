package main

import (
	"fmt"
	"io/ioutil"
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

const (
	defaultContainerMode = "init"
	annotationsFilePath  = "/conjur/podinfo/annotations"
	secretsBasePath      = "/conjur/secrets"
)

var annotationsMap map[string]string

var envAnnotationsConversion = map[string]string{
	"CONJUR_AUTHN_LOGIN":  "conjur.org/authn-identity",
	"CONTAINER_MODE":      "conjur.org/container-mode",
	"SECRETS_DESTINATION": "conjur.org/secrets-destination",
	"K8S_SECRETS":         "conjur.org/k8s-secrets",
	"RETRY_COUNT_LIMIT":   "conjur.org/retry-count-limit",
	"RETRY_INTERVAL_SEC":  "conjur.org/retry-interval-sec",
	"DEBUG":               "conjur.org/debug-logging",
}

func main() {
	log.Info(messages.CSPFK008I, secrets.FullVersionName)

	// Only attempt to populate from annotations if the annotations file exists
	// TODO: Figure out strategy for dealing with explicit annotation file path
	// set by user. In that case we can't just ignore that the file is missing.
	if _, err := os.Stat(annotationsFilePath); err == nil {
		annotationsMap, err = annotations.NewAnnotationsFromFile(annotationsFilePath)
		if err != nil {
			printErrorAndExit(messages.CSPFK040E)
		}

		errLogs, infoLogs := secretsConfigProvider.ValidateAnnotations(annotationsMap)
		logErrorsAndConditionalExit(errLogs, infoLogs, messages.CSPFK049E)
	}

	// Initialize Authenticator and Secrets Provider configurations
	authnConfig := setupAuthnConfig()
	validateContainerMode(authnConfig.ContainerMode)
	secretsConfig := setupSecretsConfig()

	// Initialize a Conjur Secret Fetcher
	secretRetriever, err := conjur.NewConjurSecretRetriever(*authnConfig)
	if err != nil {
		printErrorAndExit(err.Error())
	}

	providerConfig := secrets.ProviderConfig{
		StoreType:          secretsConfig.StoreType,
		PodNamespace:       secretsConfig.PodNamespace,
		RequiredK8sSecrets: secretsConfig.RequiredK8sSecrets,
		SecretFileBasePath: secretsBasePath,
		AnnotationsMap:     annotationsMap,
	}
	provideSecrets, errs := secrets.NewProviderForType(
		secretRetriever.Retrieve,
		providerConfig,
	)
	logErrorsAndConditionalExit(errs, nil, messages.CSPFK053E)

	provideSecrets = secrets.RetryableSecretProvider(
		time.Duration(secretsConfig.RetryIntervalSec)*time.Second,
		secretsConfig.RetryCountLimit,
		provideSecrets,
	)

	err = provideSecrets()
	if err != nil {
		printErrorAndExit(fmt.Sprintf(messages.CSPFK039E, secretsConfig.StoreType, err.Error()))
	}
}

func setupAuthnConfig() *authnConfigProvider.Config {
	// Provides a custom env for authenticator settings retrieval.
	// Log the origin of settings which have multiple possible sources.
	customEnv := func(key string) string {
		if annotation, ok := envAnnotationsConversion[key]; ok {
			if value := annotationsMap[annotation]; value != "" {
				log.Info(messages.CSPFK014I, key, fmt.Sprintf("annotation %s", annotation))
				return value
			}

			if value := os.Getenv(key); value == "" && key == "CONTAINER_MODE" {
				log.Info(messages.CSPFK014I, key, "default")
				return defaultContainerMode
			}

			log.Info(messages.CSPFK014I, key, "environment")
		}

		return os.Getenv(key)
	}

	log.Info(messages.CSPFK013I)
	authnSettings := authnConfigProvider.GatherSettings(customEnv)

	errLogs := authnSettings.Validate(ioutil.ReadFile)
	logErrorsAndConditionalExit(errLogs, nil, messages.CSPFK008E)

	return authnSettings.NewConfig()
}

func setupSecretsConfig() *secretsConfigProvider.Config {
	secretsProviderSettings := secretsConfigProvider.GatherSecretsProviderSettings(annotationsMap)

	errLogs, infoLogs := secretsConfigProvider.ValidateSecretsProviderSettings(secretsProviderSettings)
	logErrorsAndConditionalExit(errLogs, infoLogs, messages.CSPFK015E)

	return secretsConfigProvider.NewConfig(secretsProviderSettings)
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
