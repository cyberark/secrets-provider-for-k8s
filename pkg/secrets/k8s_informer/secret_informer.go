package k8sinformer

import (
	"fmt"
	"sync"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
)

const (
	SecretEventTypeAdd    = "added"
	SecretEventTypeUpdate = "updated"
)

// SecretEvent contains information about a secret change event
type SecretEvent struct {
	Secret    *v1.Secret
	EventType string // "added", "updated"
}

// SecretEventNotifier sends a notification when a secret event occurs
// This interface allows the informer to trigger the secrets provider
type SecretEventNotifier interface {
	NotifySecretEvent(event SecretEvent) bool
}

// SecretInformer wraps a Kubernetes Secret informer and notifies on secret events
type SecretInformer struct {
	clientset kubernetes.Interface
	namespace string
	factory   informers.SharedInformerFactory
	informer  cache.SharedIndexInformer
	stopCh    chan struct{}
	notifier  SecretEventNotifier
	queue     workqueue.TypedInterface[SecretEvent]
	stopOnce  sync.Once
	workerWg  sync.WaitGroup
}

// NewSecretInformer creates a new SecretInformer for the given namespace
func NewSecretInformer(
	clientset kubernetes.Interface,
	namespace string,
	notifier SecretEventNotifier,
) *SecretInformer {
	return &SecretInformer{
		clientset: clientset,
		namespace: namespace,
		stopCh:    make(chan struct{}),
		notifier:  notifier,
		queue:     workqueue.NewTyped[SecretEvent](),
	}
}

// Start initializes and starts the secret informer
func (si *SecretInformer) Start() error {
	if si.notifier == nil {
		return fmt.Errorf(messages.CSPFK076E)
	}

	// Create a factory for the given namespace
	si.factory = informers.NewSharedInformerFactoryWithOptions(
		si.clientset,
		0, // Default resync period
		informers.WithNamespace(si.namespace),
	)

	// Get the secret informer
	secretInformer := si.factory.Core().V1().Secrets().Informer()
	si.informer = secretInformer

	// Register event handlers
	// Note: We don't need to handle onDelete because any Conjur secrets would have been removed
	// on deletion of the K8s secret. Plus the next call to provideSecrets() will operate on a
	// fresh snapshot of labeled K8s secrets so there's no stale state to clean up.
	_, err := secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    si.onAdd,
		UpdateFunc: si.onUpdate,
	})
	if err != nil {
		return fmt.Errorf(messages.CSPFK078E, err)
	}

	// Start the informer (non-blocking call)
	si.factory.Start(si.stopCh)

	// Cache sync runs in background. Handlers check HasSynced() before processing events.
	go func() {
		if !cache.WaitForCacheSync(si.stopCh, secretInformer.HasSynced) {
			log.Error(messages.CSPFK079E)
			si.Stop()
		}
	}()

	// Start the worker goroutine to process queued events if any
	si.workerWg.Add(1)
	go si.runWorker()

	log.Info(messages.CSPFK026I, si.namespace)
	return nil
}

// Stop gracefully stops the secret informer
func (si *SecretInformer) Stop() {
	si.stopOnce.Do(func() {
		// Shutdown queue first so worker exits via processNextItem
		si.queue.ShutDown()
		// Then close stopCh to signal factory and other goroutines
		close(si.stopCh)
		// Shutdown the informer factory to clean up goroutines
		if si.factory != nil {
			si.factory.Shutdown()
		}
		log.Info(messages.CSPFK027I, si.namespace)
	})
	// Wait for runWorker to fully exit
	si.workerWg.Wait()
}

// hasManagedByProviderLabel checks if a secret has the required label
func (si *SecretInformer) hasManagedByProviderLabel(secret *v1.Secret) bool {
	if secret == nil || secret.Labels == nil {
		return false
	}
	return utils.IsTrue(secret.Labels[config.ManagedByProviderKey])
}

// onAdd is called when a new secret is added
func (si *SecretInformer) onAdd(obj interface{}) {
	// Skip events during initial sync or if informer not set
	if si.informer == nil || !si.informer.HasSynced() {
		return
	}

	secret, ok := obj.(*v1.Secret)
	if !ok {
		return
	}

	if si.hasManagedByProviderLabel(secret) {
		// Add event to queue for asynchronous processing
		event := SecretEvent{
			Secret:    secret.DeepCopy(),
			EventType: SecretEventTypeAdd,
		}
		si.queue.Add(event)
	}
}

// conjurMapChanged checks if the conjur-map value changed on an updated K8s Secret
func (si *SecretInformer) conjurMapChanged(oldSecret, newSecret *v1.Secret) bool {
	// Handle nil Data fields - treat as changed if one is nil and other is not
	oldDataNil := oldSecret.Data == nil
	newDataNil := newSecret.Data == nil
	if oldDataNil || newDataNil {
		return oldDataNil != newDataNil
	}

	oldMap := oldSecret.Data[config.ConjurMapKey]
	newMap := newSecret.Data[config.ConjurMapKey]

	// Compare the conjur-map values
	if len(oldMap) != len(newMap) {
		return true
	}

	for i := range oldMap {
		if oldMap[i] != newMap[i] {
			return true
		}
	}

	return false
}

// labelsChanged checks if the managed-by-provider label changed
func (si *SecretInformer) labelsChanged(oldSecret, newSecret *v1.Secret) bool {
	oldHasLabel := si.hasManagedByProviderLabel(oldSecret)
	newHasLabel := si.hasManagedByProviderLabel(newSecret)
	return oldHasLabel != newHasLabel
}

// onUpdate is called when an existing secret is updated
func (si *SecretInformer) onUpdate(oldObj, newObj interface{}) {
	// Skip events during initial sync or if informer not set
	if si.informer == nil || !si.informer.HasSynced() {
		return
	}

	newSecret, newOk := newObj.(*v1.Secret)
	if !newOk {
		log.Warn(messages.CSPFK074E)
		return
	}

	oldSecret, oldOk := oldObj.(*v1.Secret)
	if !oldOk {
		log.Warn(messages.CSPFK075E)
		return
	}

	// Only process updates if:
	// 1. The secret has the managed-by-provider label (either old or new)
	// 2. AND either the conjur-map changed OR the label changed
	// This prevents circular updates: when the secrets provider updates secret values (Data field),
	// it doesn't change conjur-map or labels, so we ignore those updates.
	hasRelevantLabel := si.hasManagedByProviderLabel(oldSecret) || si.hasManagedByProviderLabel(newSecret)
	if hasRelevantLabel && (si.conjurMapChanged(oldSecret, newSecret) || si.labelsChanged(oldSecret, newSecret)) {
		// Add event to queue for asynchronous processing
		event := SecretEvent{
			Secret:    newSecret.DeepCopy(),
			EventType: SecretEventTypeUpdate,
		}
		si.queue.Add(event)
	}
}

// runWorker continuously processes events from the queue until stop signals
func (si *SecretInformer) runWorker() {
	defer func() {
		si.workerWg.Done()
		if r := recover(); r != nil {
			log.Error(messages.CSPFK083E, r)
		}
		log.Info(messages.CSPFK030I)
	}()

	log.Info(messages.CSPFK029I)
	for {
		// processNextItem returns false only when queue is shut down
		// or stopCh is closed (after queue.ShutDown in Stop())
		if !si.processNextItem() {
			return
		}
	}
}

// processNextItem retrieves and processes the next item from the queue
// Returns false only when the queue is shut down
func (si *SecretInformer) processNextItem() bool {
	item, quit := si.queue.Get()
	if quit {
		return false
	}

	defer si.queue.Done(item)

	err := si.processEvent(item)
	if err != nil {
		// Log error and discard the event
		// Note: the processEvent uses a notifier with built-in timeout to send the event,
		//       so we don't need to re-queue the event on failure. Just log and leave it
		//       to the next K8s secret event to trigger another update or periodic refresh.
		log.Error(messages.CSPFK081E, item.EventType, err)
	}

	return true
}

// processEvent sends the event to the notifier
func (si *SecretInformer) processEvent(event SecretEvent) error {
	// NotifySecretEvent has built-in 3-second timeout
	if !si.notifier.NotifySecretEvent(event) {
		return fmt.Errorf(messages.CSPFK082E)
	}

	return nil
}
