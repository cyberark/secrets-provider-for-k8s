package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	authnConfigProvider "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	v1 "k8s.io/api/core/v1"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/k8s"
	secretsConfigProvider "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
)

const defaultAnnotationFilePath = "/conjur/podinfo/annotations"
const defaultFileSecretsBasePath = "/conjur/secrets"

func main() {
	var annotationFilePath string
	var fileSecretsBasePath string
	var testMode bool
	flag.StringVar(&annotationFilePath, "f", defaultAnnotationFilePath, "path to annotation file")
	flag.StringVar(&fileSecretsBasePath, "s", defaultFileSecretsBasePath, "base path for push to file secrets")
	flag.BoolVar(&testMode, "test", false, "running in test mode where deps are mocked")
	flag.Parse()

	log.Info(messages.CSPFK008I, secrets.FullVersionName)

	// Initialize authn configuration
	authnConfig, err := authnConfigProvider.NewFromEnv()
	if err != nil {
		printErrorAndExit(messages.CSPFK008E)
	}

	// Parse annotations
	annotationsMap := map[string]string{}

	log.Debug("Checking for annotations file at %q", annotationFilePath)
	// Only attempt to populate from annotations if the annotations file exists
	_, err = os.Stat(annotationFilePath)
	if err != nil && !os.IsExist(err) {
		log.Debug(fmt.Sprintf("Skipped annotations file: %s", err))
	}

	if err == nil {
		log.Debug("Parsing annotations file")
		annotationsMap, err = annotations.NewAnnotationsFromFile(annotationFilePath)
		if err != nil {
			printErrorAndExit(messages.CSPFK040E)
		}
	}

	errLogs, infoLogs := secretsConfigProvider.ValidateAnnotations(annotationsMap)
	logErrorsAndConditionalExit(errLogs, infoLogs, messages.CSPFK049E)

	secretsProviderSettings := secretsConfigProvider.GatherSecretsProviderSettings(annotationsMap)
	errLogs, infoLogs = secretsConfigProvider.ValidateSecretsProviderSettings(secretsProviderSettings)
	logErrorsAndConditionalExit(errLogs, infoLogs, messages.CSPFK015E)

	// Combine annotations and secretsProviderSettings
	// TODO: fix this hack which is used to make sure the same map is sent everywhere
	for k, v := range secretsProviderSettings {
		annotationsMap[k] = v
	}

	// Initialize Secrets Provider configuration
	secretsConfig := secretsConfigProvider.NewConfig(annotationsMap)

	log.Info("Secrets Provider using store type=%q", secretsConfig.StoreType)
	// Define dependencies
	var retrieveK8sSecret k8s.RetrieveK8sSecretFunc
	var updateK8sSecret k8s.UpdateK8sSecretFunc
	var fetchSecrets conjur.FetchSecretsFunc

	// TODO: maybe a better way to manage dependencies is to have a registry
	// 	instead of threading them down through methods that couldn't care less
	if testMode {
		retrieveK8sSecret = func(namespace string, secretName string) (*v1.Secret, error) {
			log.Debug(
				`Retrieving k8s secret with
<< namespace=%s
<< secretName=%s
`,
				namespace,
				secretName,
			)
			return &v1.Secret{
				Data: map[string][]byte{
					"conjur-map": []byte(
						fmt.Sprintf(`
key1: path/to/key1/in/conjur/from/secret/%s/in/namespace/%s
`, secretName, namespace,
						),
					),
				},
			}, nil
		}
		updateK8sSecret = func(namespace string, secretName string, originalK8sSecret *v1.Secret, stringDataEntriesMap map[string][]byte) error {
			log.Info(
				`Updating k8s secret with
<< namespace=%s
<< secretName=%s
<< originalK8sSecret.Data["conjur-map"]=%s
<< stringDataEntriesMap["key1"]=%s
`,
				namespace,
				secretName,
				string(originalK8sSecret.Data["conjur-map"]),
				string(stringDataEntriesMap["key1"]),
			)
			return nil
		}
		fetchSecrets = func(variableIDs []string) (map[string][]byte, error) {
			// To make go lint happy
			if authnConfig.ClientCertRetryCountLimit == 0 {}

			log.Info(
				`Fetching Conjurs secret with
<< variableIDs=%#v`, variableIDs,
			)

			var out = map[string][]byte{}
			for _, id := range variableIDs {
				out[id] = []byte("value-"+id)
			}
			return out, nil
		}
	} else {
		retrieveK8sSecret = k8s.RetrieveK8sSecret
		updateK8sSecret = k8s.UpdateK8sSecret
		log.Debug("Creating Conjur Secret Fetcher")
		secretFetcher, err := conjur.NewConjurSecretFetcher(*authnConfig)
		if err != nil {
			printErrorAndExit(err.Error())
		}
		fetchSecrets = secretFetcher.Fetch
	}

	// Select Provider
	provideSecrets, err := secrets.NewProviderForType(
		retrieveK8sSecret,
		updateK8sSecret,
		fileSecretsBasePath,
		secretsConfig.StoreType,
		annotationsMap,
	)
	if err != nil {
		printErrorAndExit(fmt.Sprintf(messages.CSPFK014E, err.Error()))
	}

	log.Debug("Selected provider for store type %q", secretsConfig.StoreType)

	// Make Provider retriable
	provideSecrets = secrets.RetriableSecretProvider(
		time.Duration(secretsConfig.RetryIntervalSec)*time.Second,
		secretsConfig.RetryCountLimit,
		provideSecrets,
	)

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
