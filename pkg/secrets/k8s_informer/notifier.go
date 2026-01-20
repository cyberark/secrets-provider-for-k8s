package k8sinformer

import (
	"time"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// ChannelNotifier sends events to a channel
type ChannelNotifier struct {
	eventsChan chan<- SecretEvent
}

// NewChannelNotifier creates a notifier that sends events to a channel
func NewChannelNotifier(eventsChan chan<- SecretEvent) *ChannelNotifier {
	return &ChannelNotifier{
		eventsChan: eventsChan,
	}
}

// NotifySecretEvent sends the event to the channel with 3-second timeout
// Returns true if sent successfully, false if timeout or channel is nil
func (n *ChannelNotifier) NotifySecretEvent(event SecretEvent) bool {
	if n.eventsChan == nil {
		log.Error(messages.CSPFK084I)
		return false
	}

	select {
	case n.eventsChan <- event:
		return true
	case <-time.After(3 * time.Second):
		return false
	}
}
