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
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

// ProviderFunc describes a function type responsible for providing secrets to an unspecified target.
type ProviderFunc func() error

// NewProviderForType returns a ProviderFunc responsible for retrieving secrets
// and either updating K8s secrets or writing them to the filesystem.
func NewProviderForType(
	retrievek8sSecret k8s.RetrieveK8sSecretFunc,
	updatek8sSecret k8s.UpdateK8sSecretFunc,
	secretsRetrieverFunc conjur.RetrieveSecretsFunc,
	storeType string,
	podNamespace string,
	requiredK8sSecrets []string,
	settings map[string]string,
) (ProviderFunc, []error) {
	switch storeType {
	case config.K8s:
		provider := k8s_secrets_storage.NewProvider(
			retrievek8sSecret,
			updatek8sSecret,
			secretsRetrieverFunc,
			requiredK8sSecrets,
			podNamespace,
		)
		return provider.Provide, nil
	case config.File:
		provider, err := pushtofile.NewProvider(
			secretsRetrieverFunc,
			settings,
		)
		if err != nil {
			return nil, err
		}
		return provider.Provide, nil
	default:
		return nil, []error{fmt.Errorf(
			messages.CSPFK054E,
			storeType,
		)}
	}
}

// RetryableSecretProvider returns a new ProviderFunc, which wraps the provided ProviderFunc
// in a limitedBackOff-restricted Retry call.
func RetryableSecretProvider(
	retryInterval time.Duration,
	retryCountLimit int,
	provideSecrets ProviderFunc,
) ProviderFunc {
	limitedBackOff := utils.NewLimitedBackOff(
		retryInterval,
		retryCountLimit,
	)

	return func() error {
		err := backoff.Retry(func() error {
			if limitedBackOff.RetryCount() > 0 {
				log.Info(fmt.Sprintf(messages.CSPFK010I, limitedBackOff.RetryCount(), limitedBackOff.RetryLimit))
			}

			return provideSecrets()
		}, limitedBackOff)

		if err != nil {
			log.Error(messages.CSPFK038E, err)
		}
		return err
	}
}
