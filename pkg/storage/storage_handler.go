package storage

import (
	"fmt"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/file"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/memory"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	secretsConfigProvider "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/config"
	secretsHandlers "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/handlers"
	storageConfigProvider "github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/storage/config"
)

type StorageHandler struct {
	AccessToken    access_token.AccessToken
	SecretsHandler secretsHandlers.SecretsHandler
}

func NewStorageHandler(storageConfig storageConfigProvider.Config) (*StorageHandler, error) {
	var infoLogger = log.InfoLogger

	var accessToken access_token.AccessToken
	var secretsHandler secretsHandlers.SecretsHandler
	var err error

	if storageConfig.StoreType == storageConfigProvider.K8S {
		infoLogger.Printf(fmt.Sprintf(log.CSPFK001I, storageConfigProvider.K8S))

		secretsConfig, err := secretsConfigProvider.NewFromEnv()
		if err != nil {
			return nil, log.RecorderError(log.CSPFK003E)
		}

		accessToken, err = memory.NewAccessToken()
		if err != nil {
			return nil, log.RecorderError(log.CSPFK004E)
		}

		secretsHandler, err = secretsHandlers.NewSecretHandlerK8sUseCase(*secretsConfig, accessToken)
		if err != nil {
			return nil, log.RecorderError(log.CSPFK001E)
		}
	} else if storageConfig.StoreType == storageConfigProvider.None {
		accessToken, err = file.NewAccessToken(storageConfig.TokenFilePath)
		if err != nil {
			return nil, log.RecorderError(log.CSPFK002E)
		}

		var secretHandlerNoneUseCase secretsHandlers.SecretHandlerNoneUseCase
		secretsHandler = &secretHandlerNoneUseCase
	} else {
		// although this is checked when creating `storageConfig.StoreType` we check this here for code clarity and future dev guard
		return nil, log.RecorderError(log.CSPFK005E, storageConfig.StoreType)
	}

	return &StorageHandler{
		AccessToken:    accessToken,
		SecretsHandler: secretsHandler,
	}, nil
}
