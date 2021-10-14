package secrets

import (
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/k8s"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/push_to_file"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

func RetriableSecretProvider(
	retryInterval time.Duration,
	retryCountLimit int,
	provideSecrets ProviderFunc,
) ProviderFunc {
	limitedBackOff := utils.NewLimitedBackOff(
		retryInterval,
		retryCountLimit,
	)

	return func(fetchSecrets conjur.FetchSecretsFunc) error {
		err := backoff.Retry(func() error {
			if limitedBackOff.RetryCount() > 0 {
				log.Info(fmt.Sprintf(messages.CSPFK010I, limitedBackOff.RetryCount(), limitedBackOff.RetryLimit))
			}

			return provideSecrets(fetchSecrets)
		}, limitedBackOff)

		if err != nil {
			log.Error(messages.CSPFK038E, err)
		}
		return err
	}
}

type ProviderFunc func(fetchSecrets conjur.FetchSecretsFunc) error

func NewProviderForType(
	// deps
	retrievek8sSecret k8s.RetrieveK8sSecretFunc,
	updatek8sSecret k8s.UpdateK8sSecretFunc,
	fileSecretsBasePath string,
	// args
	storeType string,
	settings map[string]string,
) (ProviderFunc, error) {
	switch storeType {
	case config.K8s:
		return k8s_secrets_storage.NewProvider(
			retrievek8sSecret,
			updatek8sSecret,
			settings,
		).Provide, nil
	case config.File:
		provider, err := push_to_file.NewProvider(
			fileSecretsBasePath,
			settings,
		)
		if err != nil {
			return nil, err
		}
		return provider.Provide, nil
	}

	return nil, fmt.Errorf(
		"unabe to initialize secrets provider unrecognised store type: %s",
		storeType,
	)
}
