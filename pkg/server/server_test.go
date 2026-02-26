package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerHealthAndReadiness(t *testing.T) {
	server, err := NewServer("")
	require.NoError(t, err)
	server.Start()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	baseURL := "http://" + server.Address()
	client := &http.Client{Timeout: 200 * time.Millisecond}

	assert.Eventually(t, func() bool {
		resp, reqErr := client.Get(baseURL + "/healthz")
		if reqErr != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 20*time.Millisecond)

	assert.Eventually(t, func() bool {
		resp, reqErr := client.Get(baseURL + "/readyz")
		if reqErr != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusServiceUnavailable
	}, 2*time.Second, 20*time.Millisecond)

	server.SetReady(true)

	assert.Eventually(t, func() bool {
		resp, reqErr := client.Get(baseURL + "/readyz")
		if reqErr != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 20*time.Millisecond)
}
