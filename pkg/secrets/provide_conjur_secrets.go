package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	k8sSecretsStorage "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

// ProviderConfig provides the configuration necessary to create a secrets
// Provider.
type ProviderConfig struct {
	// Config common to all providers
	StoreType string

	// Config specific to Kubernetes Secrets provider
	PodNamespace       string
	RequiredK8sSecrets []string

	// Config specific to Push to File provider
	SecretFileBasePath   string
	TemplateFileBasePath string
	AnnotationsMap       map[string]string
}

// ProviderFunc describes a function type responsible for providing secrets to an unspecified target.
type ProviderFunc func() error

// NewProviderForType returns a ProviderFunc responsible for providing secrets in a given mode.
func NewProviderForType(
	traceContext context.Context,
	secretsRetrieverFunc conjur.RetrieveSecretsFunc,
	providerConfig ProviderConfig,
) (ProviderFunc, []error) {
	switch providerConfig.StoreType {
	case config.K8s:
		provider := k8sSecretsStorage.NewProvider(
			traceContext,
			secretsRetrieverFunc,
			providerConfig.RequiredK8sSecrets,
			providerConfig.PodNamespace,
		)
		return provider.Provide, nil
	case config.File:
		provider, err := pushtofile.NewProvider(
			secretsRetrieverFunc,
			providerConfig.SecretFileBasePath,
			providerConfig.TemplateFileBasePath,
			providerConfig.AnnotationsMap,
		)
		if err != nil {
			return nil, err
		}
		provider.SetTraceContext(traceContext)
		return provider.Provide, nil
	default:
		return nil, []error{fmt.Errorf(
			messages.CSPFK054E,
			providerConfig.StoreType,
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

var TickerQuit = make(chan bool)
var ProviderDone = make(chan bool)
var SecretProviderError = make(chan error)
func PeriodicSecretProvider(
	secretRefreshInterval time.Duration,
	mode string,
	provideSecrets ProviderFunc,
	) ProviderFunc {
	return func() error {
		err := provideSecrets()
		if err != nil {
			SecretProviderError <- err
			return err
		}
		switch {
		case mode != "sidecar":
			SecretProviderError <- err
			return err
		case mode == "sidecar" && secretRefreshInterval > 0:
			ticker := time.NewTicker(secretRefreshInterval)
			go periodicSecretProvider(provideSecrets, ticker)
		default:
			// fall through
		}
		// Wait here
		select {
		case <-ProviderDone: // Todo change to SIGTERM or SIGINTERRUPT
		}
		if mode == "sidecar" && secretRefreshInterval > 0{
			TickerQuit <- true
			time.Sleep(1 * time.Second) // give a second for the go routine to shut down
		}
		return nil
	}
}

func periodicSecretProvider(provideSecrets ProviderFunc, ticker *time.Ticker) {
	for {
		select {
		case <-ticker.C:
			err := provideSecrets()
			if err != nil {
				SecretProviderError <- err
				ProviderDone <- true
			}
			case <-TickerQuit:
			ticker.Stop()
			return
		}
	}
}
