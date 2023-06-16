package entrypoint

import (
	"context"
	"fmt"
	"os"
	"time"

	authnConfigProvider "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	spLog "github.com/cyberark/secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	secretsConfigProvider "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	k8sSecretsStorage "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
)

const (
	defaultContainerMode       = "init"
	defaultAnnotationsFilePath = "/conjur/podinfo/annotations"
	defaultSecretsBasePath     = "/conjur/secrets"
	defaultTemplatesBasePath   = "/conjur/templates"
	tracerName                 = "secrets-provider"
	tracerService              = "secrets-provider"
	tracerEnvironment          = "production"
	tracerID                   = 1
)

var envAnnotationsConversion = map[string]string{
	"CONJUR_AUTHN_LOGIN":     "conjur.org/authn-identity",
	"CONTAINER_MODE":         "conjur.org/container-mode",
	"SECRETS_DESTINATION":    "conjur.org/secrets-destination",
	"K8S_SECRETS":            "conjur.org/k8s-secrets",
	"RETRY_COUNT_LIMIT":      "conjur.org/retry-count-limit",
	"RETRY_INTERVAL_SEC":     "conjur.org/retry-interval-sec",
	"DEBUG":                  "conjur.org/debug-logging",
	"JAEGER_COLLECTOR_URL":   "conjur.org/jaeger-collector-url",
	"LOG_TRACES":             "conjur.org/log-traces",
	"JWT_TOKEN_PATH":         "conjur.org/jwt-token-path",
	"REMOVE_DELETED_SECRETS": "conjur.org/remove-deleted-secrets-enabled",
}

func StartSecretsProvider() {
	exitCode := startSecretsProviderWithDeps(
		defaultAnnotationsFilePath,
		defaultSecretsBasePath,
		defaultTemplatesBasePath,
		conjur.NewSecretRetriever,
		secrets.NewProviderForType,
		secrets.NewStatusUpdater,
	)
	os.Exit(exitCode)
}

func startSecretsProviderWithDeps(
	annotationsFilePath string,
	secretsBasePath string,
	templatesBasePath string,
	retrieverFactory conjur.RetrieverFactory,
	providerFactory secrets.ProviderFactory,
	statusUpdaterFactory secrets.StatusUpdaterFactory,
) (exitCode int) {
	exitCode = 0

	logError := func(errStr string) {
		log.Error(errStr)
		exitCode = 1
	}

	log.Info(messages.CSPFK008I, secrets.FullVersionName)

	// Create a TracerProvider, Tracer, and top-level (parent) Span
	tracerType, tracerURL := getTracerConfig(annotationsFilePath)
	ctx, tracer, deferFunc, err := createTracer(tracerType, tracerURL)
	defer deferFunc(ctx)
	if err != nil {
		logError(err.Error())
		return
	}

	// Process Pod Annotations
	annotationsMap, err := processAnnotations(ctx, tracer, annotationsFilePath)
	if err != nil {
		logError(err.Error())
		return
	}

	// Setup Required Configurations
	secretsConfig, authnConfig, providerConfig, err := setupConfigs(
		ctx,
		tracer,
		secretsBasePath,
		templatesBasePath,
		annotationsMap,
	)
	if err != nil {
		logError(err.Error())
		return
	}

	// Create a Conjur Secret Retriever
	secretRetriever, err := secretRetriever(ctx, tracer, annotationsMap, authnConfig, retrieverFactory)
	if err != nil {
		logError(err.Error())
		return
	}

	// Create a Secrets Provider
	provideSecrets, errs := secretsProvider(ctx, tracer, secretRetriever, providerConfig, providerFactory)
	if err = spLog.LogErrorsAndInfos(errs, nil); err != nil {
		logError(messages.CSPFK053E)
		return
	}

	// Wrap Secrets Provider in Retry Logic
	provideSecrets = secrets.RetryableSecretProvider(
		time.Duration(secretsConfig.RetryIntervalSec)*time.Second,
		secretsConfig.RetryCountLimit,
		provideSecrets,
	)

	if err = secrets.RunSecretsProvider(
		secrets.ProviderRefreshConfig{
			Mode:                  getContainerMode(annotationsMap),
			SecretRefreshInterval: secretsConfig.SecretsRefreshInterval,
			// Create a channel to send a quit signal to the periodic secret provider.
			// TODO: Currently, this is just used for testing, but in the future we
			// may want to create a SIGTERM or SIGHUP handler to catch a signal from
			// a user / external entity, and then send an (empty struct) quit signal
			// on this channel to trigger a graceful shut down of the Secrets Provider.
			ProviderQuit: make(chan struct{}),
		},
		provideSecrets,
		statusUpdaterFactory(),
	); err != nil {
		logError(err.Error())
	}
	return
}

func processAnnotations(ctx context.Context, tracer trace.Tracer, annotationsFilePath string) (map[string]string, error) {
	// Only attempt to populate from annotations if the annotations file exists
	// TODO: Figure out strategy for dealing with explicit annotation file path
	// set by user. In that case we can't just ignore that the file is missing.
	if _, err := os.Stat(annotationsFilePath); err != nil {
		return nil, nil
	}

	_, span := tracer.Start(ctx, "Process Annotations")
	defer span.End()

	return annotations.NewAnnotationsFromFile(annotationsFilePath)
}

func secretRetriever(
	ctx context.Context,
	tracer trace.Tracer,
	annotationsMap map[string]string,
	authnConfig *authnConfigProvider.Configuration,
	retrieverFactory conjur.RetrieverFactory,
) (conjur.RetrieveSecretsFunc, error) {
	_, span := tracer.Start(ctx, "Initialize a Conjur Secret Retriever")
	defer span.End()

	return retrieverFactory(*authnConfig)
}

func secretsProvider(
	ctx context.Context,
	tracer trace.Tracer,
	secretRetriever conjur.RetrieveSecretsFunc,
	providerConfig *secrets.ProviderConfig,
	providerFactory secrets.ProviderFactory,
) (secrets.ProviderFunc, []error) {
	_, span := tracer.Start(ctx, "Initialize a Secrets Provider")
	defer span.End()

	return providerFactory(ctx, secretRetriever, *providerConfig)
}

func customEnv(annotationsMap map[string]string) func(key string) string {
	return func(key string) string {
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
}

func getContainerMode(annotationsMap map[string]string) string {
	containerMode := "init"
	if mode, exists := annotationsMap[secretsConfigProvider.ContainerModeKey]; exists {
		containerMode = mode
	} else if mode = os.Getenv("CONTAINER_MODE"); mode == "sidecar" || mode == "application" {
		containerMode = mode
	}
	return containerMode
}

func setupConfigs(
	ctx context.Context,
	tracer trace.Tracer,
	secretsBasePath string,
	templatesBasePath string,
	annotationsMap map[string]string,
) (
	*secretsConfigProvider.Config,
	*authnConfigProvider.Configuration,
	*secrets.ProviderConfig,
	error,
) {
	// Setup Secrets Provider configuration
	_, spSpan := tracer.Start(ctx, "Setup Secrets Provider Configuration")
	secretsConfig, err := secretsConfigProvider.NewConfigFromEnvironmentAndAnnotations(annotationsMap)
	spSpan.End()
	if err != nil {
		spSpan.RecordErrorAndSetStatus(err)
		log.Error(messages.CSPFK008E)
		return nil, nil, nil, err
	}

	// Setup AuthnK8s configuration
	_, authnSpan := tracer.Start(ctx, "Setup AuthnK8s Configuration")
	authnConfig, err := authnConfigProvider.NewConfigFromCustomEnv(
		os.ReadFile,
		customEnv(annotationsMap),
	)
	authnSpan.End()
	if err != nil {
		authnSpan.RecordErrorAndSetStatus(err)
		log.Error(messages.CSPFK008E)
		return secretsConfig, nil, nil, err
	}

	// Initialize Provider-specific configuration
	_, provSpan := tracer.Start(ctx, "Setup Provider-Specific Configuration")
	providerConfig := secrets.ProviderConfig{
		CommonProviderConfig: secrets.CommonProviderConfig{
			StoreType:       secretsConfig.StoreType,
			SanitizeEnabled: secretsConfig.SanitizeEnabled,
		},
		K8sProviderConfig: k8sSecretsStorage.K8sProviderConfig{
			PodNamespace:       secretsConfig.PodNamespace,
			RequiredK8sSecrets: secretsConfig.RequiredK8sSecrets,
		},
		P2FProviderConfig: pushtofile.P2FProviderConfig{
			SecretFileBasePath:   secretsBasePath,
			TemplateFileBasePath: templatesBasePath,
			AnnotationsMap:       annotationsMap,
		},
	}
	provSpan.End()

	return secretsConfig, &authnConfig, &providerConfig, nil
}
