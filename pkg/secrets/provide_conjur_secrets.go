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

const (
	secretProviderGracePeriod = time.Duration(10 * time.Millisecond)
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

// SecretProvider returns a new ProviderFunc, which wraps a retryable
// ProviderFunc inside a function that operates in one of three modes:
//  - Run once and return (for init or application container modes)
//  - Run once and sleep forever (for sidecar mode without periodic refresh)
//  - Run periodically (for sidecar mode with periodic refresh)
func SecretProvider(
	secretRefreshInterval time.Duration,
	mode string,
	provideSecrets ProviderFunc,
	providerQuit chan struct{},
) ProviderFunc {

	var periodicQuit = make(chan struct{})
	var periodicError = make(chan error)
	var ticker *time.Ticker

	return func() error {
		err := provideSecrets()
		switch {
		case err != nil:
			// Return immediately upon error, regardless of operating mode
			return err
		case mode != "sidecar":
			// Run once and return if not in sidecar mode
			return err
		case secretRefreshInterval > 0:
			// Run periodically if in sidecar mode with periodic refresh
			ticker = time.NewTicker(secretRefreshInterval)
			go periodicSecretProvider(provideSecrets, ticker,
				periodicQuit, periodicError)
		default:
			// Run once and sleep forever if in sidecar mode without
			// periodic refresh (fall through)
		}

		// Wait here for a signal to quit providing secrets or an error
		// from the periodicSecretProvider() function
		select {
		case <-providerQuit:
			break
		case err = <-periodicError:
			break
		}

		// Allow the periodicSecretProvider goroutine to gracefully shut down
		if secretRefreshInterval > 0 {
			// Kill the ticker
			ticker.Stop()
			periodicQuit <- struct{}{}
			// Let the go routine exit
			time.Sleep(secretProviderGracePeriod)
		}
		return err
	}
}

func periodicSecretProvider(
	provideSecrets ProviderFunc,
	ticker *time.Ticker,
	periodicQuit <-chan struct{},
	periodicError chan<- error,
) {
	for {
		select {
		case <-periodicQuit:
			return
		case <-ticker.C:
			err := provideSecrets()
			if err != nil {
				periodicError <- err
			}
		}
	}
}
