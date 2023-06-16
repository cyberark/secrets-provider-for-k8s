package entrypoint

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
	k8sSecretsStorage "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
	"go.opentelemetry.io/otel/attribute"
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

	// Gather K8s authenticator config and create a Conjur secret retriever
	secretRetriever, err := secretRetriever(ctx, tracer, annotationsMap, retrieverFactory)
	if err != nil {
		logError(err.Error())
		return
	}

	provideSecrets, secretsConfig, err := secretsProvider(
		ctx,
		tracer,
		secretsBasePath,
		templatesBasePath,
		annotationsMap,
		secretRetriever,
		providerFactory,
	)
	if err != nil {
		logError(err.Error())
		return
	}

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
	annotationsMap, err := annotations.NewAnnotationsFromFile(annotationsFilePath)
	if err != nil {
		log.Error(err.Error())
		span.RecordErrorAndSetStatus(err)
		return nil, err
	}

	errLogs, infoLogs := secretsConfigProvider.ValidateAnnotations(annotationsMap)
	if err := logErrorsAndInfos(errLogs, infoLogs); err != nil {
		log.Error(messages.CSPFK049E)
		span.RecordErrorAndSetStatus(errors.New(messages.CSPFK049E))
		return nil, err
	}

	return annotationsMap, nil
}

func secretRetriever(
	ctx context.Context,
	tracer trace.Tracer,
	annotationsMap map[string]string,
	retrieverFactory conjur.RetrieverFactory,
) (conjur.RetrieveSecretsFunc, error) {
	// Gather authenticator config
	_, span := tracer.Start(ctx, "Gather authenticator config")
	defer span.End()

	authnConfig, err := authnConfigProvider.NewConfigFromCustomEnv(
		ioutil.ReadFile,
		customEnv(annotationsMap),
	)
	if err != nil {
		span.RecordErrorAndSetStatus(err)
		log.Error(messages.CSPFK008E)
		return nil, err
	}

	// Initialize a Conjur secret retriever
	secretRetriever, err := retrieverFactory(authnConfig)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	return secretRetriever, nil
}

func secretsProvider(
	ctx context.Context,
	tracer trace.Tracer,
	secretsBasePath string,
	templatesBasePath string,
	annotationsMap map[string]string,
	secretRetriever conjur.RetrieveSecretsFunc,
	providerFactory secrets.ProviderFactory,
) (secrets.ProviderFunc, *secretsConfigProvider.Config, error) {
	_, span := tracer.Start(ctx, "Create single-use secrets provider")
	defer span.End()

	// Initialize Secrets Provider configuration
	secretsConfig, err := setupSecretsConfig(annotationsMap)
	if err != nil {
		log.Error(err.Error())
		span.RecordErrorAndSetStatus(err)
		return nil, nil, err
	}
	providerConfig := &secrets.ProviderConfig{
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

	// Tag the span with the secrets provider mode
	span.SetAttributes(attribute.String("store_type", secretsConfig.StoreType))

	// Create a secrets provider
	provideSecrets, errs := providerFactory(ctx,
		secretRetriever, *providerConfig)
	if err := logErrorsAndInfos(errs, nil); err != nil {
		log.Error(messages.CSPFK053E)
		span.RecordErrorAndSetStatus(errors.New(messages.CSPFK053E))
		return nil, nil, err
	}

	return provideSecrets, secretsConfig, nil
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

func setupSecretsConfig(annotationsMap map[string]string) (*secretsConfigProvider.Config, error) {
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

func getContainerMode(annotationsMap map[string]string) string {
	containerMode := "init"
	if mode, exists := annotationsMap[secretsConfigProvider.ContainerModeKey]; exists {
		containerMode = mode
	} else if mode = os.Getenv("CONTAINER_MODE"); mode == "sidecar" || mode == "application" {
		containerMode = mode
	}
	return containerMode
}
