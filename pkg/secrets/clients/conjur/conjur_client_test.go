package conjur

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewConjurClient tests the creation of a new Conjur client
func TestNewConjurClient(t *testing.T) {
	t.Run("Successfully creates a new client", func(t *testing.T) {
		t.Setenv("CONJUR_APPLIANCE_URL", "https://conjur.example.com")
		t.Setenv("CONJUR_ACCOUNT", "test")

		client, err := NewConjurClient([]byte("test-token"))

		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("Fails to create client with invalid config", func(t *testing.T) {
		t.Setenv("CONJUR_APPLIANCE_URL", "")
		t.Setenv("CONJUR_ACCOUNT", "")

		client, err := NewConjurClient([]byte("test-token"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Must specify an ApplianceURL")
		assert.Nil(t, client)
	})
}
