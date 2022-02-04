package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	authnConfigProvider "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	secretsConfigProvider "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultContainerMode = "init"
	annotationsFilePath  = "/conjur/podinfo/annotations"
	secretsBasePath      = "/conjur/secrets"
	templatesBasePath    = "/conjur/templates"
	tracerName           = "secrets-provider"
	tracerService        = "secrets-provider"
	tracerEnvironment    = "production"
	tracerID             = 1
)

var annotationsMap map[string]string

var envAnnotationsConversion = map[string]string{
	"CONJUR_AUTHN_LOGIN":   "conjur.org/authn-identity",
	"CONTAINER_MODE":       "conjur.org/container-mode",
	"SECRETS_DESTINATION":  "conjur.org/secrets-destination",
	"K8S_SECRETS":          "conjur.org/k8s-secrets",
	"RETRY_COUNT_LIMIT":    "conjur.org/retry-count-limit",
	"RETRY_INTERVAL_SEC":   "conjur.org/retry-interval-sec",
	"DEBUG":                "conjur.org/debug-logging",
	"JAEGER_COLLECTOR_URL": "conjur.org/jaeger-collector-url",
	"LOG_TRACES":           "conjur.org/log-traces",
	"JWT_TOKEN_PATH":       "conjur.org/jwt-token-path",
}

func main() {
	// os.Exit() does not call deferred functions, so defer exit until after
	// all other deferred functions have been called.
	exitCode := 0
	defer func() { os.Exit(exitCode) }()

	logError := func(errStr string) {
		log.Error(errStr)
		exitCode = 1
	}

	log.Info(messages.CSPFK008I, secrets.FullVersionName)

	// Create a TracerProvider, Tracer, and top-level (parent) Span
	tracerType, tracerURL := getTracerConfig()
	ctx, tracer, deferFunc, err := createTracer(tracerType, tracerURL)
	defer deferFunc(ctx)
	if err != nil {
		logError(err.Error())
		return
	}

	// Process Pod Annotations
	if err := processAnnotations(ctx, tracer); err != nil {
		logError(err.Error())
		return
	}

	// Gather K8s authenticator config and create a Conjur secret retriever
	secretRetriever, err := secretRetriever(ctx, tracer)
	if err != nil {
		logError(err.Error())
		return
	}

	// Gather secrets config and create a retryable Secrets Provider
	provideSecrets, secretsConfig, err := retryableSecretsProvider(ctx, tracer, secretRetriever)
	if err != nil {
		logError(err.Error())
		return
	}

	// Provide secrets
	err = provideSecrets()
	if err != nil {
		errStr := fmt.Sprintf(messages.CSPFK039E, secretsConfig.StoreType, err.Error())
		logError(errStr)
	}
}

func processAnnotations(ctx context.Context, tracer trace.Tracer) error {
	// Only attempt to populate from annotations if the annotations file exists
	// TODO: Figure out strategy for dealing with explicit annotation file path
	// set by user. In that case we can't just ignore that the file is missing.
	if _, err := os.Stat(annotationsFilePath); err == nil {
		_, span := tracer.Start(ctx, "Process Annotations")
		defer span.End()
		annotationsMap, err = annotations.NewAnnotationsFromFile(annotationsFilePath)
		if err != nil {
			log.Error(err.Error())
			span.RecordErrorAndSetStatus(err)
			return err
		}

		errLogs, infoLogs := secretsConfigProvider.ValidateAnnotations(annotationsMap)
		if err := logErrorsAndInfos(errLogs, infoLogs); err != nil {
			log.Error(messages.CSPFK049E)
			span.RecordErrorAndSetStatus(errors.New(messages.CSPFK049E))
			return err
		}
	}
	return nil
}

func secretRetriever(ctx context.Context,
	tracer trace.Tracer) (*conjur.SecretRetriever, error) {
	// Gather authenticator config
	_, span := tracer.Start(ctx, "Gather authenticator config")
	defer span.End()

	authnConfig, err := authnConfigProvider.NewConfigFromCustomEnv(ioutil.ReadFile, customEnv)
	if err != nil {
		span.RecordErrorAndSetStatus(err)
		log.Error(messages.CSPFK008E)
		return nil, err
	}
	if err = validateContainerMode(authnConfig.GetContainerMode()); err != nil {
		span.RecordErrorAndSetStatus(err)
		log.Error(err.Error())
		return nil, err
	}

	// Initialize a Conjur secret retriever
	secretRetriever, err := conjur.NewSecretRetriever(authnConfig)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	return secretRetriever, nil
}

func retryableSecretsProvider(
	ctx context.Context,
	tracer trace.Tracer,
	secretRetriever *conjur.SecretRetriever) (secrets.ProviderFunc, *secretsConfigProvider.Config, error) {

	_, span := tracer.Start(ctx, "Create retryable secrets provider")
	defer span.End()

	// Initialize Secrets Provider configuration
	secretsConfig, err := setupSecretsConfig()
	if err != nil {
		log.Error(err.Error())
		span.RecordErrorAndSetStatus(err)
		return nil, nil, err
	}
	providerConfig := &secrets.ProviderConfig{
		StoreType:            secretsConfig.StoreType,
		PodNamespace:         secretsConfig.PodNamespace,
		RequiredK8sSecrets:   secretsConfig.RequiredK8sSecrets,
		SecretFileBasePath:   secretsBasePath,
		TemplateFileBasePath: templatesBasePath,
		AnnotationsMap:       annotationsMap,
	}

	// Tag the span with the secrets provider mode
	span.SetAttributes(attribute.String("store_type", secretsConfig.StoreType))

	// Create a secrets provider
	provideSecrets, errs := secrets.NewProviderForType(ctx,
		secretRetriever.Retrieve, *providerConfig)
	if err := logErrorsAndInfos(errs, nil); err != nil {
		log.Error(messages.CSPFK053E)
		span.RecordErrorAndSetStatus(errors.New(messages.CSPFK053E))
		return nil, nil, err
	}

	provideSecrets = secrets.RetryableSecretProvider(
		time.Duration(secretsConfig.RetryIntervalSec)*time.Second,
		secretsConfig.RetryCountLimit,
		provideSecrets,
	)
	return provideSecrets, secretsConfig, nil
}

func customEnv(key string) string {
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

func setupSecretsConfig() (*secretsConfigProvider.Config, error) {
	secretsProviderSettings := secretsConfigProvider.GatherSecretsProviderSettings(annotationsMap)

	errLogs, infoLogs := secretsConfigProvider.ValidateSecretsProviderSettings(secretsProviderSettings)
	if err := logErrorsAndInfos(errLogs, infoLogs); err != nil {
		log.Error(messages.CSPFK015E)
		return nil, err
	}

	return secretsConfigProvider.NewConfig(secretsProviderSettings), nil
}

func logErrorsAndInfos(errLogs []error, infoLogs []error) error {
	for _, err := range infoLogs {
		log.Info(err.Error())
	}
	if len(errLogs) > 0 {
		for _, err := range errLogs {
			log.Error(err.Error())
		}
		return errors.New("fatal errors occurred, check Secrets Provider logs")
	}
	return nil
}

func validateContainerMode(containerMode string) error {
	validContainerModes := []string{
		"init",
		"application",
	}

	for _, validContainerModeType := range validContainerModes {
		if containerMode == validContainerModeType {
			return nil
		}
	}
	return fmt.Errorf(messages.CSPFK007E, containerMode, validContainerModes)
}
