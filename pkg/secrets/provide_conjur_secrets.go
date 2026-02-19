package secrets

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	k8sinformer "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_informer"
	k8sSecretsStorage "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

const (
	secretProviderGracePeriod = time.Duration(10 * time.Millisecond)
	informerDebounceDelay     = time.Duration(200 * time.Millisecond)
	informerDebounceMaxDelay  = time.Duration(10 * time.Second)
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
type ProviderFunc func() (updated bool, err error)

// RepeatableProviderFunc describes a function type that is capable of looping
// indefinitely while providing secrets to unspecified targets.
type RepeatableProviderFunc func() error

// ProviderFactory defines a function type for creating a ProviderFunc given a
// RetrieveSecretsFunc and ProviderConfig.
type ProviderFactory func(traceContent context.Context, secretsRetrieverFunc conjur.RetrieveSecretsFunc, providerConfig ProviderConfig) (ProviderFunc, []error)

// K8sProviderInstance holds a reference to the K8s provider so we can dynamically manage its secrets
var K8sProviderInstance *k8sSecretsStorage.K8sProvider

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
		// Store a reference to the K8s provider so it can be accessed by the informer handler
		K8sProviderInstance = &provider
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

	return func() (bool, error) {
		var updated bool
		var retErr error

		op := func() error {
			updated, retErr = provideSecrets()
			return retErr
		}

		notify := func(err error, next time.Duration) {
			retries := limitedBackOff.RetryCount()
			limit := "unlimited"
			if limitedBackOff.RetryLimit >= 0 {
				limit = fmt.Sprintf("%d", limitedBackOff.RetryLimit)
			}
			log.Warn(fmt.Sprintf(messages.CSPFK040E, next, retries, limit, err))
		}

		err := backoff.RetryNotify(op, limitedBackOff, notify)
		if err != nil {
			log.Error(messages.CSPFK038E, err)
			return updated, err
		}
		return updated, nil
	}
}

// ProviderRefreshConfig specifies the secret refresh configuration
// for a repeatable secret provider.
type ProviderRefreshConfig struct {
	Mode                  string
	SecretRefreshInterval time.Duration
	ProviderQuit          chan struct{}
	InformerEvents        <-chan k8sinformer.SecretEvent
}

// RunSecretsProvider takes a retryable ProviderFunc, and runs it in one of four modes:
//   - Run once and return (for init or application container modes)
//   - Run once and sleep forever (for sidecar mode without periodic refresh)
//   - Run periodically (for sidecar mode with periodic refresh)
//   - Run on informer events (for sidecar mode with secret informer)
func RunSecretsProvider(
	config ProviderRefreshConfig,
	provideSecrets ProviderFunc,
	status StatusUpdater,
) error {

	var periodicQuit = make(chan struct{})
	var periodicError = make(chan error)
	var ticker *time.Ticker
	var err error

	if err = status.CopyScripts(); err != nil {
		return err
	}
	if _, err = provideSecrets(); err != nil {
		// Return immediately upon error, regardless of operating mode
		return err
	}
	err = status.SetSecretsProvided()
	if err != nil {
		return err
	}
	switch {
	case config.Mode != "sidecar":
		// Run once and return if not in sidecar mode
		return nil
	default:
		// In sidecar mode, we can run both informer and periodic refresh simultaneously
		// Start informer-triggered provider if informer events channel is provided
		if config.InformerEvents != nil {
			informerCfg := informerConfig{
				informerEvents: config.InformerEvents,
				periodicQuit:   periodicQuit,
				periodicError:  periodicError,
			}
			go informerTriggeredProvider(provideSecrets, informerCfg, status)
		}
		// Start periodic refresh if interval is set
		if config.SecretRefreshInterval > 0 {
			ticker = time.NewTicker(config.SecretRefreshInterval)
			periodicCfg := periodicConfig{
				ticker:        ticker,
				periodicQuit:  periodicQuit,
				periodicError: periodicError,
			}
			go periodicSecretProvider(provideSecrets, periodicCfg, status)
		}
		// If neither informer nor periodic refresh is configured,
		// fall through to sleep forever
	}

	// Wait here for a signal to quit providing secrets or an error
	// from the periodicSecretProvider() or informerTriggeredProvider() function
	if config.SecretRefreshInterval > 0 || config.InformerEvents != nil {
		// Wait on both quit signal and error channel if goroutines are running
		select {
		case <-config.ProviderQuit:
			break
		case err = <-periodicError:
			break
		}

		// Allow the background goroutines to gracefully shut down
		// Kill the ticker if running
		if ticker != nil {
			ticker.Stop()
		}
		// Close the channel so all goroutines listening to it will receive the signal
		close(periodicQuit)
		// Let the goroutines exit
		time.Sleep(secretProviderGracePeriod)
	} else {
		// If no goroutines are running (no periodic refresh, no informer),
		// wait for OS termination signal to keep the sidecar container running
		log.Debug(messages.CSPFK012D)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigChan)
		select {
		case <-config.ProviderQuit:
			break
		case sig := <-sigChan:
			log.Debug(fmt.Sprintf("Received signal %v, shutting down gracefully", sig))
			break
		}
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
) {
	for {
		select {
		case <-config.periodicQuit:
			return
		case <-config.ticker.C:
			updated, err := provideSecrets()
			if err == nil && updated {
				err = status.SetSecretsUpdated()
			}
			if err != nil {
				config.periodicError <- err
			}
		}
	}
}

type informerConfig struct {
	informerEvents <-chan k8sinformer.SecretEvent
	periodicQuit   <-chan struct{}
	periodicError  chan<- error
}

func informerTriggeredProvider(
	provideSecrets ProviderFunc,
	config informerConfig,
	status StatusUpdater,
) {
	var debounceTimer *time.Timer
	var timerChan <-chan time.Time
	var eventCount int
	var firstEventTime time.Time
	// Accumulate keys to remove across multiple UPDATE events
	keysToRemoveCumulative := make(map[string][]string)

	runProvideSecrets := func() {
		if eventCount > 1 {
			log.Info(messages.CSPFK031I, eventCount)
		}

		var updated bool
		var err error
		// ProvideWithCleanup is not wrapped with retry logic in the way the provideSecrets func is, so it will only
		// attempt once if keys need removing
		if len(keysToRemoveCumulative) > 0 {
			updated, err = K8sProviderInstance.ProvideWithCleanup(keysToRemoveCumulative)
			keysToRemoveCumulative = make(map[string][]string)
		} else {
			updated, err = provideSecrets()
		}
		if err == nil && updated {
			err = status.SetSecretsUpdated()
		}
		if err != nil {
			config.periodicError <- err
		}
		debounceTimer = nil
		timerChan = nil
		eventCount = 0
	}

	for {
		select {
		case <-config.periodicQuit:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		case event := <-config.informerEvents:
			log.Info(messages.CSPFK028I, event.EventType, event.Secret.Namespace, event.Secret.Name)
			eventCount++
			if eventCount == 1 {
				firstEventTime = time.Now()
			}

			// If it is an update event, update the cumulative keys to remove
			if event.EventType == k8sinformer.SecretEventTypeUpdate {
				// Merge re-added keys: if a key was removed then re-added before the debounce
				// fires, it should not be in the cleanup list
				keysToRemoveCumulative = K8sProviderInstance.MergeReAddedKeys(keysToRemoveCumulative, event.OldSecret, event.Secret)

				// Accumulate keys that were removed from the conjur-map for batch cleanup
				keysToRemove := K8sProviderInstance.GetRemovedKeys(event.OldSecret, event.Secret)
				for secretName, keys := range keysToRemove {
					keysToRemoveCumulative[secretName] = append(keysToRemoveCumulative[secretName], keys...)
				}
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			// Run immediately if we've already exceeded the max delay
			if time.Since(firstEventTime) >= informerDebounceMaxDelay {
				log.Info(messages.CSPFK035I, eventCount)
				runProvideSecrets()
				continue
			}
			debounceTimer = time.NewTimer(informerDebounceDelay)
			timerChan = debounceTimer.C
		case <-timerChan:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			runProvideSecrets()
		}
	}
}
