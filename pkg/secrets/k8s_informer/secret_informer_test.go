package k8sinformer

import (
	"context"
	"testing"
	"time"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// MockSecretEventNotifier is a mock implementation of SecretEventNotifier for testing
type MockSecretEventNotifier struct {
	events []SecretEvent
}

func (m *MockSecretEventNotifier) NotifySecretEvent(event SecretEvent) bool {
	m.events = append(m.events, event)
	return true
}

func TestSecretInformerAddEvents(t *testing.T) {
	tests := []struct {
		name           string
		secretName     string
		labels         map[string]string
		expectedEvents []string
	}{
		{
			name:       "secret with managed-by-provider label triggers added event",
			secretName: "managed-secret",
			labels: map[string]string{
				config.ManagedByProviderKey: "true",
			},
			expectedEvents: []string{"added"},
		},
		{
			name:           "secret without label does not trigger event",
			secretName:     "unmanaged-secret",
			labels:         nil,
			expectedEvents: []string{},
		},
		{
			name:           "secret with nil labels does not trigger event",
			secretName:     "no-labels-secret",
			labels:         nil,
			expectedEvents: []string{},
		},
		{
			name:           "secret with empty labels does not trigger event",
			secretName:     "empty-labels-secret",
			labels:         map[string]string{},
			expectedEvents: []string{},
		},
		{
			name:       "secret with wrong label value does not trigger event",
			secretName: "wrong-value-secret",
			labels: map[string]string{
				config.ManagedByProviderKey: "false",
			},
			expectedEvents: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			namespace := "test-namespace"
			mockNotifier := &MockSecretEventNotifier{}
			informer := NewSecretInformer(clientset, namespace, mockNotifier)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := informer.Start()
			require.NoError(t, err, "Failed to start informer")
			defer informer.Stop()

			time.Sleep(100 * time.Millisecond)

			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.secretName,
					Namespace: namespace,
					Labels:    tt.labels,
				},
				Data: map[string][]byte{
					"password": []byte("secret-value"),
				},
			}

			_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create secret")

			time.Sleep(500 * time.Millisecond)

			assert.Len(t, mockNotifier.events, len(tt.expectedEvents), "Number of events does not match expected")

			for i, expectedEvent := range tt.expectedEvents {
				if i < len(mockNotifier.events) {
					if mockNotifier.events[i].EventType != expectedEvent {
						t.Errorf("Expected event %d to be '%s', got '%s'", i, expectedEvent, mockNotifier.events[i].EventType)
					}
					if mockNotifier.events[i].Secret.Name != tt.secretName {
						t.Errorf("Expected event %d secret name to be '%s', got '%s'", i, tt.secretName, mockNotifier.events[i].Secret.Name)
					}
				}
			}
		})
	}
}

func TestSecretInformerUpdateEvents(t *testing.T) {
	tests := []struct {
		name           string
		initialSecret  *v1.Secret
		updateSecret   func(*v1.Secret)
		expectedEvents []string
		description    string
	}{
		{
			name: "label added triggers updated event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unmanaged-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				s.Labels = map[string]string{
					config.ManagedByProviderKey: "true",
				}
			},
			expectedEvents: []string{"updated"},
			description:    "Adding the managed-by-provider label should trigger UPDATE event",
		},
		{
			name: "label removed does not trigger updated event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "true",
					},
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				s.Labels = nil
			},
			expectedEvents: []string{"added"},
			description:    "Removing the managed-by-provider label should not trigger UPDATE event",
		},
		{
			name: "label set to false does not trigger updated event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "true",
					},
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				s.Labels[config.ManagedByProviderKey] = "false"
			},
			expectedEvents: []string{"added"},
			description:    "Set the managed-by-provider label to false should not trigger UPDATE event",
		},
		{
			name: "conjur-map added triggers updated event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "true",
					},
				},
				Data: map[string][]byte{},
			},
			updateSecret: func(s *v1.Secret) {
				s.Data[config.ConjurMapKey] = []byte("SECRET: secrets/test_secret")
			},
			expectedEvents: []string{"added", "updated"},
			description:    "Adding conjur-map should trigger UPDATE event",
		},
		{
			name: "conjur-map removed triggers updated event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "true",
					},
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				delete(s.Data, config.ConjurMapKey)
			},
			expectedEvents: []string{"added", "updated"},
			description:    "Removing conjur-map should trigger UPDATE event",
		},
		{
			name: "update conjur-map without label does not trigger event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unmanaged-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				s.Data[config.ConjurMapKey] = []byte("SECRET: secrets/new_secret")
			},
			expectedEvents: []string{},
			description:    "Updates to conjur_map without label should not trigger events",
		},
		{
			name: "update conjur-map with label set to false does not trigger event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unmanaged-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "false",
					},
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				s.Data[config.ConjurMapKey] = []byte("SECRET: secrets/new_secret")
			},
			expectedEvents: []string{},
			description:    "Updates to conjur_map with the label set to false should not trigger events",
		},
		{
			name: "conjur-map change triggers updated event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "true",
					},
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				s.Data[config.ConjurMapKey] = []byte("SECRET: secrets/new_secret")
			},
			expectedEvents: []string{"added", "updated"},
			description:    "Changing conjur-map should trigger UPDATE event",
		},
		{
			name: "data-only update does not trigger event",
			initialSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "true",
					},
				},
				Data: map[string][]byte{
					config.ConjurMapKey: []byte("SECRET: secrets/test_secret"),
					"SECRET":            []byte("original-value"),
				},
			},
			updateSecret: func(s *v1.Secret) {
				s.Data["SECRET"] = []byte("updated-value")
			},
			expectedEvents: []string{"added"},
			description:    "Data-only updates should be ignored to prevent circular updates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			namespace := "test-namespace"
			mockNotifier := &MockSecretEventNotifier{}
			informer := NewSecretInformer(clientset, namespace, mockNotifier)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := informer.Start()
			require.NoError(t, err, "Failed to start informer")
			defer informer.Stop()

			time.Sleep(100 * time.Millisecond)

			// Create the initial secret
			secret := tt.initialSecret.DeepCopy()
			secret.Namespace = namespace
			_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create secret")

			time.Sleep(200 * time.Millisecond)

			// Update the secret
			updatedSecret := secret.DeepCopy()
			tt.updateSecret(updatedSecret)
			_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, updatedSecret, metav1.UpdateOptions{})
			require.NoError(t, err, "Failed to update secret")

			time.Sleep(200 * time.Millisecond)

			assert.Len(t, mockNotifier.events, len(tt.expectedEvents), "%s: Number of events does not match expected", tt.description)
			for i, e := range mockNotifier.events {
				t.Logf("Event %d: %s - %s", i, e.EventType, e.Secret.Name)
			}

			for i, expectedEvent := range tt.expectedEvents {
				if i < len(mockNotifier.events) {
					if mockNotifier.events[i].EventType != expectedEvent {
						t.Errorf("%s: Expected event %d to be '%s', got '%s'", tt.description, i, expectedEvent, mockNotifier.events[i].EventType)
					}
				}
			}
		})
	}
}

func TestSecretInformerEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		notifier    SecretEventNotifier
		secret      *v1.Secret
		errExpected bool
	}{
		{
			name:     "Informer handles nil notifier gracefully",
			notifier: nil,
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						config.ManagedByProviderKey: "true",
					},
				},
				Data: map[string][]byte{
					"password": []byte("secret-value"),
				},
			},
			errExpected: true,
		},
		{
			name:     "secret with nil labels should not trigger events",
			notifier: &MockSecretEventNotifier{},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-labels-secret",
					Namespace: "test-namespace",
					Labels:    nil,
				},
				Data: map[string][]byte{
					"password": []byte("secret-value"),
				},
			},
			errExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			namespace := "test-namespace"
			informer := NewSecretInformer(clientset, namespace, tt.notifier)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := informer.Start()
			if tt.errExpected {
				require.Error(t, err, "Expected error when starting informer")
				return
			}
			require.NoError(t, err, "Failed to start informer")
			defer informer.Stop()

			time.Sleep(100 * time.Millisecond)

			secret := tt.secret.DeepCopy()
			secret.Namespace = namespace
			_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create secret")

			time.Sleep(200 * time.Millisecond)

			// Test passes if no panic occurs
			if tt.notifier != nil {
				mockNotifier := tt.notifier.(*MockSecretEventNotifier)
				if tt.secret.Labels == nil || tt.secret.Labels[config.ManagedByProviderKey] != "true" {
					assert.Len(t, mockNotifier.events, 0, "%s: Expected 0 events", tt.name)
				}
			}
		})
	}
}

func TestSecretInformerMultipleSecrets(t *testing.T) {
	// Test that multiple secrets are handled correctly
	clientset := fake.NewSimpleClientset()
	namespace := "test-namespace"
	mockNotifier := &MockSecretEventNotifier{}
	informer := NewSecretInformer(clientset, namespace, mockNotifier)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := informer.Start()
	require.NoError(t, err, "Failed to start informer")
	defer informer.Stop()

	time.Sleep(100 * time.Millisecond)

	secrets := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "managed-secret-1",
			labels: map[string]string{
				config.ManagedByProviderKey: "true",
			},
		},
		{
			name:   "unmanaged-secret",
			labels: nil,
		},
		{
			name: "managed-secret-2",
			labels: map[string]string{
				config.ManagedByProviderKey: "true",
			},
		},
	}

	for _, s := range secrets {
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.name,
				Namespace: namespace,
				Labels:    s.labels,
			},
			Data: map[string][]byte{
				"password": []byte("secret-value"),
			},
		}

		_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create secret %s", s.name)
	}

	time.Sleep(500 * time.Millisecond)

	// Should have 2 events (one for each managed secret)
	assert.Len(t, mockNotifier.events, 2, "Expected 2 events")

	// Verify both managed secrets triggered events
	eventNames := make(map[string]bool)
	for _, event := range mockNotifier.events {
		if event.EventType != "added" {
			t.Errorf("Expected 'added' event, got '%s'", event.EventType)
		}
		eventNames[event.Secret.Name] = true
	}

	if !eventNames["managed-secret-1"] {
		t.Error("Expected event for managed-secret-1")
	}
	if !eventNames["managed-secret-2"] {
		t.Error("Expected event for managed-secret-2")
	}
	if eventNames["unmanaged-secret"] {
		t.Error("Unexpected event for unmanaged-secret")
	}
}
