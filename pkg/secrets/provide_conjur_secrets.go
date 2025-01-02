package secrets

import (
	"context"
	"fmt"
	k8swebhooks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_webhooks"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	secretsConfigProvider "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	k8sSecretsStorage "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

const (
	secretProviderGracePeriod = time.Duration(10 * time.Millisecond)
)

// CommonProviderConfig provides config that is common to all providers
type CommonProviderConfig struct {
	StoreType       string
	SanitizeEnabled bool
}

// ProviderConfig provides the configuration necessary to create a secrets
// Provider.
type ProviderConfig struct {
	CommonProviderConfig
	k8sSecretsStorage.K8sProviderConfig
	pushtofile.P2FProviderConfig
}

// ProviderFunc describes a function type responsible for providing secrets to
// an unspecified target. It returns either an error, or a flag that indicates
// whether any target secret files or Kubernetes Secrets have been updated.
type ProviderFunc func(secrets ...string) (updated bool, err error)

// RepeatableProviderFunc describes a function type that is capable of looping
// indefinitely while providing secrets to unspecified targets.
type RepeatableProviderFunc func() error

// ProviderFactory defines a function type for creating a ProviderFunc given a
// RetrieveSecretsFunc and ProviderConfig.
type ProviderFactory func(traceContent context.Context, secretsRetrieverFunc conjur.RetrieveSecretsFunc, providerConfig ProviderConfig) (ProviderFunc, []error)

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
			providerConfig.CommonProviderConfig.SanitizeEnabled,
			providerConfig.K8sProviderConfig,
		)

		if providerConfig.K8sProviderConfig.IsRepeatableMode {
			k8swebhooks.StartWebhookServer(provider, providerConfig.RequiredK8sSecrets)
		}

		return provider.Provide, nil
	case config.File:
		provider, err := pushtofile.NewProvider(
			secretsRetrieverFunc,
			providerConfig.CommonProviderConfig.SanitizeEnabled,
			providerConfig.P2FProviderConfig,
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

	return func(secrets ...string) (bool, error) {
		var updated bool
		var retErr error

		err := backoff.Retry(func() error {
			if limitedBackOff.RetryCount() > 0 {
				log.Info(fmt.Sprintf(messages.CSPFK010I, limitedBackOff.RetryCount(), limitedBackOff.RetryLimit))
			}
			updated, retErr = provideSecrets(secrets...)
			return retErr
		}, limitedBackOff)

		if err != nil {
			log.Error(messages.CSPFK038E, err)
		}
		return updated, err
	}
}

// ProviderRefreshConfig specifies the secret refresh configuration
// for a repeatable secret provider.
type ProviderRefreshConfig struct {
	Mode                  string
	SecretRefreshInterval time.Duration
	ProviderQuit          chan struct{}
}

// RunSecretsProvider takes a retryable ProviderFunc, and runs it in one of three modes:
//   - Run once and return (for init or application container modes)
//   - Run once and sleep forever (for sidecar mode without periodic refresh)
//   - Run periodically (for sidecar mode with periodic refresh)
func RunSecretsProvider(
	config ProviderRefreshConfig,
	provideSecrets ProviderFunc,
	status StatusUpdater,
	providerConfig *secretsConfigProvider.Config,
) error {

	var periodicQuit = make(chan struct{})
	var periodicError = make(chan error)
	var ticker *time.Ticker
	var err error

	if err = status.CopyScripts(); err != nil {
		return err
	}
	if _, err = provideSecrets(providerConfig.RequiredK8sSecrets...); err != nil && (config.Mode != "sidecar" && config.Mode != "standalone") {
		// Return immediately upon error, regardless of operating mode
		return err
	}
	if err == nil {
		err = status.SetSecretsProvided()
		// In sidecar or standalone mode provider should keep running
		if err != nil && (config.Mode != "sidecar" && config.Mode != "standalone") {
			return err
		}
	}
	switch {
	case config.Mode != "sidecar" && config.Mode != "standalone":
		// Run once and return if not in sidecar mode
		return nil
	case config.SecretRefreshInterval > 0:
		log.Info(fmt.Sprintf(messages.CSPFK025I, config.SecretRefreshInterval))
		// Run periodically if in sidecar mode with periodic refresh
		ticker = time.NewTicker(config.SecretRefreshInterval)
		config := periodicConfig{
			ticker:        ticker,
			periodicQuit:  periodicQuit,
			periodicError: periodicError,
		}
		go periodicSecretProvider(provideSecrets, config, status, providerConfig.RequiredK8sSecrets...)
	default:
		// Run once and sleep forever if in sidecar mode without
		// periodic refresh (fall through)
	}

	// Wait here for a signal to quit providing secrets or an error
	// from the periodicSecretProvider() function
	select {
	case <-config.ProviderQuit:
		break
	case err = <-periodicError:
		//periodic provider in standalone mode should keep working event there is provision errors.
		//errors should be appropriately logged so user can see what went wrong.
		if config.Mode != "standalone" {
			break
		}
	}

	// Allow the periodicSecretProvider goroutine to gracefully shut down
	if config.SecretRefreshInterval > 0 {
		// Kill the ticker
		ticker.Stop()
		periodicQuit <- struct{}{}
		// Let the go routine exit
		time.Sleep(secretProviderGracePeriod)
	}
	return err
}

type periodicConfig struct {
	ticker        *time.Ticker
	periodicQuit  <-chan struct{}
	periodicError chan<- error
}

func periodicSecretProvider(
	provideSecrets ProviderFunc,
	config periodicConfig,
	status StatusUpdater,
	requiredK8sSecrets ...string,
) {
	for {
		select {
		case <-config.periodicQuit:
			return
		case <-config.ticker.C:
			log.Info(messages.CSPFK024I)
			updated, err := provideSecrets(requiredK8sSecrets...)
			if err == nil && updated {
				log.Debug("Periodic provider run finished")
				err = status.SetSecretsUpdated()
			}
			if err != nil {
				config.periodicError <- err
			}
		}
	}
}
