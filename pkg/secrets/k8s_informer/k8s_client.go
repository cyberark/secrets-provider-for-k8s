package k8sinformer

import (
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// NewInClusterK8sClient creates a Kubernetes client configured for in-cluster authentication
// This is the recommended way to create a client when running inside a Kubernetes pod
func NewInClusterK8sClient() (kubernetes.Interface, error) {
	// Create an in-cluster REST configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(messages.CSPFK019E, err)
		return nil, err
	}

	// Create a Kubernetes client from the configuration
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(messages.CSPFK018E, err)
		return nil, err
	}

	return clientset, nil
}
