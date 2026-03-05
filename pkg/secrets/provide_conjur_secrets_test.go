package secrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	logger "github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	k8sinformer "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_informer"
	k8sSecretsStorage "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	providerDelayMsecs      = 5
	largeProviderDelayMsecs = 50
	atomicWriteDelayMsecs   = 3
)

// call count state, so that test cases don't interfere with one another.
type mockProvider struct {
	calledCount          int
	callLatencyMsecs     time.Duration
	injectFailure        bool
	failOnCountN         int
	injectInitialFailure bool
	failUntilCountN      int
	targetsUpdated       bool
}

func (m *mockProvider) provide() (bool, error) {
	m.calledCount++
	switch {
	case m.injectFailure && (m.calledCount >= m.failOnCountN):
		return m.targetsUpdated, errors.New("Failed to Provide")
	case m.injectInitialFailure && (m.calledCount < m.failUntilCountN):
		return m.targetsUpdated, errors.New("Failed to Provide")
	case m.callLatencyMsecs > 0:
		time.Sleep(m.callLatencyMsecs * time.Millisecond)
	}
	return m.targetsUpdated, nil
}

func (m *mockProvider) count() int {
	return m.calledCount
}

func goodProvider() *mockProvider {
	return &mockProvider{}
}

func goodProviderTargetsUpdated() *mockProvider {
	return &mockProvider{targetsUpdated: true}
}

func badProvider() *mockProvider {
	return &mockProvider{injectFailure: true, failOnCountN: 1}
}

func eventualProvider(failUntilCountN int) *mockProvider {
	return &mockProvider{injectInitialFailure: true, failUntilCountN: failUntilCountN}
}

func eventualProviderTargetsUpdated(failUntilCountN int) *mockProvider {
	return &mockProvider{
		injectInitialFailure: true,
		failUntilCountN:      failUntilCountN,
		targetsUpdated:       true,
	}
}

func slowProvider(latencyMsecs time.Duration) *mockProvider {
	return &mockProvider{callLatencyMsecs: latencyMsecs}
}
func goodAtFirstThenBadProvider(failOnCountN int) *mockProvider {
	return &mockProvider{injectFailure: true, failOnCountN: failOnCountN}
}

func TestRetryableSecretProvider(t *testing.T) {
	TestCases := []struct {
		description   string
		interval      time.Duration
		limit         int
		provider      *mockProvider
		assertOn      string
		expectUpdated bool
	}{
		{
			description: "the ProviderFunc can return within the retry limit",
			interval:    time.Duration(10) * time.Millisecond,
			limit:       3,
			provider:    eventualProvider(3),
			assertOn:    "success",
		},
		{
			description:   "the ProviderFunc can return with target updated status",
			interval:      time.Duration(10) * time.Millisecond,
			limit:         3,
			provider:      eventualProviderTargetsUpdated(3),
			assertOn:      "success",
			expectUpdated: true,
		},
		{
			description: "the ProviderFunc is retried the proper amount of times",
			interval:    time.Duration(10) * time.Millisecond,
			limit:       2,
			provider:    badProvider(),
			assertOn:    "count",
		},
		{
			description: "the ProviderFunc is only retried after the proper duration",
			interval:    time.Duration(20) * time.Millisecond,
			limit:       2,
			provider:    badProvider(),
			assertOn:    "interval",
		},
		{
			description: "the ProviderFunc retries indefinitely until it succeeds when retry limit is -1",
			interval:    time.Duration(10) * time.Millisecond,
			limit:       -1,
			provider:    eventualProvider(3),
			assertOn:    "success",
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.description, func(t *testing.T) {
			var logBuffer bytes.Buffer
			logger.InfoLogger = log.New(&logBuffer, "", 0)

			retryableProvider := RetryableSecretProvider(
				tc.interval, tc.limit, tc.provider.provide, nil,
			)

			start := time.Now()
			updated, err := retryableProvider()
			duration := time.Since(start)

			logMessages := logBuffer.String()
			switch tc.assertOn {
			case "count":
				assert.NotNil(t, err)
				assert.Contains(t, logMessages,
					fmt.Sprintf("CSPFK040E Retrying"))
				assert.Contains(t, logMessages,
					fmt.Sprintf("(attempt %d of %d)", tc.limit, tc.limit))
			case "interval":
				assert.NotNil(t, err)
				maxDuration := (time.Duration(tc.limit) * time.Millisecond * tc.interval) + (time.Duration(1) * time.Millisecond)
				assert.LessOrEqual(t, duration, maxDuration)
			case "success":
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectUpdated, updated)
		})
	}
}

func TestRetryableSecretProviderOnRetry(t *testing.T) {
	t.Run("onRetry is called on each retry when provider eventually succeeds", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger.InfoLogger = log.New(&logBuffer, "", 0)

		provider := eventualProvider(3) // fails 2 times, succeeds on 3rd
		var onRetryCalls int
		var lastOnRetryErr error
		onRetry := func(err error) {
			onRetryCalls++
			lastOnRetryErr = err
		}

		retryableProvider := RetryableSecretProvider(
			10*time.Millisecond, 5, provider.provide, onRetry,
		)

		_, err := retryableProvider()
		assert.NoError(t, err)
		assert.Equal(t, 2, onRetryCalls, "onRetry should be called twice (before 2nd and 3rd attempt)")
		assert.Error(t, lastOnRetryErr)
		assert.Contains(t, lastOnRetryErr.Error(), "Failed to Provide")
	})

	t.Run("onRetry is called on each retry when provider exhausts retry limit", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger.InfoLogger = log.New(&logBuffer, "", 0)

		provider := badProvider()
		var onRetryCalls int
		onRetry := func(err error) {
			onRetryCalls++
		}

		retryableProvider := RetryableSecretProvider(
			10*time.Millisecond, 3, provider.provide, onRetry,
		)

		_, err := retryableProvider()
		assert.Error(t, err)
		assert.Equal(t, 3, onRetryCalls, "onRetry should be called 3 times (before each retry)")
	})

	t.Run("onRetry is not called when provider succeeds on first attempt", func(t *testing.T) {
		provider := goodProvider()
		var onRetryCalls int
		onRetry := func(err error) {
			onRetryCalls++
		}

		retryableProvider := RetryableSecretProvider(
			10*time.Millisecond, 3, provider.provide, onRetry,
		)

		_, err := retryableProvider()
		assert.NoError(t, err)
		assert.Equal(t, 0, onRetryCalls, "onRetry should not be called when provider succeeds immediately")
	})

	t.Run("nil onRetry does not panic", func(t *testing.T) {
		provider := eventualProvider(2)
		retryableProvider := RetryableSecretProvider(
			10*time.Millisecond, 5, provider.provide, nil,
		)

		_, err := retryableProvider()
		assert.NoError(t, err)
	})
}

func TestRunSecretsProvider(t *testing.T) {
	testCases := []struct {
		description    string
		mode           string
		interval       time.Duration
		testTime       time.Duration // total test time for all tests must be less than 3m
		expectedCount  int
		expectProvided bool
		expectUpdated  bool
		provider       *mockProvider
		assertOn       string
		targetsUpdated bool
	}{
		{
			description:    "init container, happy path",
			mode:           "init",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(250) * time.Millisecond,
			expectedCount:  1,
			expectProvided: true,
			expectUpdated:  false,
			provider:       goodProvider(),
			assertOn:       "success",
		},
		{
			description:    "sidecar container, happy path, no targets updated",
			mode:           "sidecar",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(650+atomicWriteDelayMsecs) * time.Millisecond,
			expectedCount:  7,
			expectProvided: true,
			expectUpdated:  false,
			provider:       goodProvider(),
			assertOn:       "success",
		},
		{
			description:    "standalone container, happy path, no targets updated",
			mode:           "standalone",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(650+atomicWriteDelayMsecs) * time.Millisecond,
			expectedCount:  7,
			expectProvided: true,
			expectUpdated:  false,
			provider:       goodProvider(),
			assertOn:       "success",
		},
		{
			description:    "sidecar container, happy path, targets updated",
			mode:           "sidecar",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(650+atomicWriteDelayMsecs) * time.Millisecond,
			expectedCount:  7,
			expectProvided: true,
			expectUpdated:  true,
			provider:       goodProviderTargetsUpdated(),
			assertOn:       "success",
			targetsUpdated: true,
		},
		{
			description:    "standalone container, happy path, targets updated",
			mode:           "standalone",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(650+atomicWriteDelayMsecs) * time.Millisecond,
			expectedCount:  7,
			expectProvided: true,
			expectUpdated:  true,
			provider:       goodProviderTargetsUpdated(),
			assertOn:       "success",
			targetsUpdated: true,
		},
		{
			description:    "sidecar with zero duration",
			mode:           "sidecar",
			interval:       time.Duration(0) * time.Millisecond,
			testTime:       time.Duration(250) * time.Millisecond,
			expectedCount:  1,
			expectProvided: true,
			expectUpdated:  false,
			provider:       goodProvider(),
			assertOn:       "success",
		},
		{
			description:    "application mode, happy path",
			mode:           "application",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(250) * time.Millisecond,
			expectProvided: true,
			expectUpdated:  false,
			expectedCount:  1,
			provider:       goodProvider(),
			assertOn:       "success",
		},
		{
			// The provider is slow, but still finishes within the interval time.
			// Note the ticker is started after the first provideSecrets call so
			// that delay is added to the duration
			description:    "sidecar with slow provider",
			mode:           "sidecar",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(650+providerDelayMsecs) * time.Millisecond,
			expectedCount:  7,
			expectProvided: true,
			expectUpdated:  false,
			provider:       slowProvider(providerDelayMsecs),
			assertOn:       "success",
		},
		{
			// In this test the provider takes longer to run than the
			// interval time. The Go timer will adjust the time interval due to the
			// slower receiver. Secrets Provider is expected to be called every largeProviderDelayMsecs
			// The first timer tick is delayed by duration so that is added to the test time.
			description:    "sidecar with duration less than fetch time",
			mode:           "sidecar",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(3250+10) * time.Millisecond,
			expectedCount:  7,
			expectProvided: true,
			expectUpdated:  false,
			provider:       slowProvider(largeProviderDelayMsecs * 10),
			assertOn:       "success",
		},
		{
			description:    "badProvider for init container",
			mode:           "init",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(250) * time.Millisecond,
			expectedCount:  1,
			expectProvided: false,
			expectUpdated:  false,
			provider:       badProvider(),
			assertOn:       "fail",
		},
		{
			description:    "badProvider for sidecar",
			mode:           "sidecar",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(250) * time.Millisecond,
			expectedCount:  1,
			expectProvided: false,
			expectUpdated:  false,
			provider:       badProvider(),
			assertOn:       "fail",
		},
		{
			description:    "badProvider for standalone container",
			mode:           "standalone",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(250) * time.Millisecond,
			expectedCount:  2, // In standalone mode, the provider is called once at the beginning and once more after the first failure when it retries, then it fails
			expectProvided: false,
			expectUpdated:  false,
			provider:       badProvider(),
			assertOn:       "success", // The error will be reset to nil in standalone mode so that the provider continues to be retried
		},
		{
			description:    "goodAtFirstThenBadProvider for sidecar",
			mode:           "sidecar",
			interval:       time.Duration(100) * time.Millisecond,
			testTime:       time.Duration(350) * time.Millisecond,
			expectedCount:  1,
			expectProvided: true,
			expectUpdated:  false,
			provider:       goodAtFirstThenBadProvider(2),
			assertOn:       "fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var err error

			// Create a test provider status updater
			updater, err := newTestStatusUpdater(injectErrs{})
			assert.NoError(t, err)
			defer updater.cleanup()
			fileUpdater := updater.fileUpdater

			// Construct a secret provider function
			var providerQuit = make(chan struct{})
			refreshConfig := ProviderRefreshConfig{
				Mode:                  tc.mode,
				RunOnce:               tc.mode != "sidecar" && tc.mode != "standalone",
				SecretRefreshInterval: tc.interval,
				ProviderQuit:          providerQuit,
			}

			// Create HTTP server for standalone mode
			var httpServer *server.Server
			if tc.mode == "standalone" {
				var err2 error
				httpServer, err2 = server.NewServer("127.0.0.1:0")
				assert.NoError(t, err2)
				httpServer.Start()
				defer func() {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					_ = httpServer.Shutdown(ctx)
				}()
			}

			// Run the secrets provider
			testError := make(chan error)
			go func() {
				err := RunSecretsProvider(refreshConfig, tc.provider.provide, fileUpdater, httpServer)
				testError <- err
			}()
			select {
			case err = <-testError:
				break
			case <-time.After(tc.testTime):
				providerQuit <- struct{}{}
				break
			}

			// Check results
			if err == nil {
				assert.Equal(t, tc.expectedCount, tc.provider.count())
			}

			assert.FileExists(t, updater.targetScriptFile)
			switch tc.expectProvided {
			case true:
				assert.FileExists(t, fileUpdater.providedFile)
			case false:
				assert.NoFileExists(t, fileUpdater.providedFile)
			}
			switch tc.expectUpdated {
			case true:
				assert.FileExists(t, fileUpdater.updatedFile)
			case false:
				assert.NoFileExists(t, fileUpdater.updatedFile)
			}

			switch tc.assertOn {
			case "success":
				assert.NoError(t, err)
			case "fail":
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "Failed to Provide")
			}
		})
	}
}

// TestRunSecretsProviderSidecarWithSignalHandling tests the signal handling code path
// for sidecar mode with no periodic refresh and no informer.
//
// NOTE: This test cannot fully validate the deadlock fix because:
//   - Go's deadlock detector only triggers when ALL goroutines in the program are blocked
//   - In unit tests, the test runner goroutine remains active, preventing deadlock detection
//   - The actual deadlock was only reproducible in production where RunSecretsProvider
//     runs as the only active goroutine in main()
//
// This test validates:
// - The signal handler is properly registered and responds to OS signals
// - The container stays running (blocks) until signaled
// - Graceful shutdown occurs when receiving SIGTERM/SIGINT
func TestRunSecretsProviderSidecarWithSignalHandling(t *testing.T) {
	provider := goodProvider()
	updater, err := newTestStatusUpdater(injectErrs{})
	require.NoError(t, err)
	defer updater.cleanup()

	providerQuit := make(chan struct{})
	refreshConfig := ProviderRefreshConfig{
		Mode:                  "sidecar",
		RunOnce:               false,
		SecretRefreshInterval: time.Duration(0), // No periodic refresh
		ProviderQuit:          providerQuit,
		InformerEvents:        nil, // No informer - triggers signal handler code path
	}

	// Run the secrets provider in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- RunSecretsProvider(refreshConfig, provider.provide, updater.fileUpdater, nil)
	}()

	// Give it time to complete initial provision and reach the signal handling block
	time.Sleep(200 * time.Millisecond)

	// Verify the provider is still running (blocking on signal handler)
	select {
	case <-done:
		t.Fatal("RunSecretsProvider exited unexpectedly - should be blocking on signal handler")
	case <-time.After(100 * time.Millisecond):
		// Good - it's blocking as expected
	}

	// Verify initial provision completed
	assert.Equal(t, 1, provider.count(), "Provider should be called once initially")
	assert.FileExists(t, updater.fileUpdater.providedFile)

	// Send SIGTERM to trigger graceful shutdown
	// Note: We send to current process; the signal handler in the goroutine will receive it
	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Wait for graceful shutdown
	select {
	case err := <-done:
		assert.NoError(t, err, "Should shut down gracefully without error")
	case <-time.After(2 * time.Second):
		t.Fatal("RunSecretsProvider did not respond to SIGTERM within timeout")
	}
}

func TestRunSecretsProviderStandaloneHealthEndpoints(t *testing.T) {
	provider := goodProvider()
	updater, err := newTestStatusUpdater(injectErrs{})
	require.NoError(t, err)
	defer updater.cleanup()

	// Create HTTP server
	httpServer, err := server.NewServer("127.0.0.1:0")
	require.NoError(t, err)
	httpServer.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	serverAddress := httpServer.Address()

	providerQuit := make(chan struct{})
	refreshConfig := ProviderRefreshConfig{
		Mode:                  "standalone",
		SecretRefreshInterval: 0,
		ProviderQuit:          providerQuit,
	}

	done := make(chan error, 1)
	go func() {
		done <- RunSecretsProvider(refreshConfig, provider.provide, updater.fileUpdater, httpServer)
	}()

	baseURL := "http://" + serverAddress
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
		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 20*time.Millisecond)

	providerQuit <- struct{}{}
	select {
	case runErr := <-done:
		assert.NoError(t, runErr)
	case <-time.After(2 * time.Second):
		t.Fatal("RunSecretsProvider did not stop within timeout")
	}
}

func TestRunSecretsProviderStandaloneReadinessRecoversAfterInitialFailure(t *testing.T) {
	providerCallCount := 0
	provider := func() (bool, error) {
		providerCallCount++
		if providerCallCount == 1 {
			return false, errors.New("Failed to Provide")
		}
		return true, nil
	}

	updater, err := newTestStatusUpdater(injectErrs{})
	require.NoError(t, err)
	defer updater.cleanup()

	httpServer, err := server.NewServer("127.0.0.1:0")
	require.NoError(t, err)
	httpServer.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	providerQuit := make(chan struct{})
	refreshConfig := ProviderRefreshConfig{
		Mode:                  "standalone",
		SecretRefreshInterval: 300 * time.Millisecond,
		ProviderQuit:          providerQuit,
	}

	done := make(chan error, 1)
	go func() {
		done <- RunSecretsProvider(refreshConfig, provider, updater.fileUpdater, httpServer)
	}()

	baseURL := "http://" + httpServer.Address()
	client := &http.Client{Timeout: 200 * time.Millisecond}

	// Wait for initial provision to complete (which will fail)
	time.Sleep(100 * time.Millisecond)

	// Verify readyz returns 503 after first failure
	assert.Eventually(t, func() bool {
		resp, reqErr := client.Get(baseURL + "/readyz")
		if reqErr != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusServiceUnavailable
	}, 500*time.Millisecond, 50*time.Millisecond, "readyz should return 503 after initial failure")

	// Wait for next refresh cycle (which will succeed)
	time.Sleep(400 * time.Millisecond)

	// Verify readyz recovers to 200 after successful provision
	assert.Eventually(t, func() bool {
		resp, reqErr := client.Get(baseURL + "/readyz")
		if reqErr != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 1*time.Second, 50*time.Millisecond, "readyz should return 200 after recovery")

	providerQuit <- struct{}{}
	select {
	case runErr := <-done:
		assert.NoError(t, runErr)
	case <-time.After(3 * time.Second):
		t.Fatal("RunSecretsProvider did not stop within timeout")
	}
}

func TestNewProviderForType(t *testing.T) {
	t.Run("Returns error for unknown provider type", func(t *testing.T) {
		_, err := NewProviderForType(t.Context(), nil, ProviderConfig{
			CommonProviderConfig: CommonProviderConfig{
				StoreType: "unknown_type",
			},
		})
		require.NotNil(t, err)
		assert.Contains(t, err[0].Error(), "CSPFK054E")
	})

	t.Run("Returns file provider for 'file' type", func(t *testing.T) {
		provider, err := NewProviderForType(t.Context(), nil, ProviderConfig{
			CommonProviderConfig: CommonProviderConfig{
				StoreType: "file",
			},
			P2FProviderConfig: pushtofile.P2FProviderConfig{},
		})
		require.Nil(t, err)
		assert.NotNil(t, provider)
	})

	t.Run("Returns k8s provider for 'k8s_secrets' type", func(t *testing.T) {
		provider, err := NewProviderForType(t.Context(), nil, ProviderConfig{
			CommonProviderConfig: CommonProviderConfig{
				StoreType: "k8s_secrets",
			},
			K8sProviderConfig: k8sSecretsStorage.K8sProviderConfig{},
		})
		require.Nil(t, err)
		assert.NotNil(t, provider)
	})
}

func TestInformerTriggeredProviderBatching(t *testing.T) {
	tests := []struct {
		name                string
		batches             []int // Batch sizes - each batch is a series of rapid events, then we wait before triggering the next batch
		expectedCallCount   int
		expectedBatchCounts []int // Expected batch counts logged (nil means no batch logs expected)
	}{
		{
			name:                "single event triggers one call and no batch log",
			batches:             []int{1},
			expectedCallCount:   1,
			expectedBatchCounts: nil,
		},
		{
			name:                "multiple rapid events batched into one call",
			batches:             []int{5},
			expectedCallCount:   1,
			expectedBatchCounts: []int{5},
		},
		{
			name:                "many rapid events batched into one call",
			batches:             []int{100},
			expectedCallCount:   1,
			expectedBatchCounts: []int{100},
		},
		{
			name:                "events spaced apart trigger separate calls and no batch logs",
			batches:             []int{1, 1, 1},
			expectedCallCount:   3,
			expectedBatchCounts: nil,
		},
		{
			name:                "multiple batches with multiple events each",
			batches:             []int{5, 5, 5},
			expectedCallCount:   3,
			expectedBatchCounts: []int{5, 5, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			logger.InfoLogger = log.New(&logBuffer, "", 0)

			mockProv := goodProvider()
			updater, err := newTestStatusUpdater(injectErrs{})
			require.NoError(t, err)
			defer updater.cleanup()

			// Setup informer
			eventsChan := make(chan k8sinformer.SecretEvent, 100)
			periodicQuit := make(chan struct{})
			periodicError := make(chan error, 1)
			config := informerConfig{
				informerEvents: eventsChan,
				periodicQuit:   periodicQuit,
				periodicError:  periodicError,
				onSetReady:     func(error) {},
			}
			go informerTriggeredProvider(mockProv.provide, config, updater.fileUpdater)

			eventIndex := 0
			for batchIdx, batchSize := range tt.batches {
				// Send all events in this batch rapidly
				for i := 0; i < batchSize; i++ {
					eventsChan <- k8sinformer.SecretEvent{
						Secret: &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      fmt.Sprintf("secret-%d", eventIndex),
								Namespace: "test-namespace",
							},
						},
						EventType: k8sinformer.SecretEventTypeAdd,
					}
					eventIndex++
				}

				// Wait before sending next batch (except after the last batch)
				if batchIdx < len(tt.batches)-1 {
					time.Sleep(informerDebounceDelay + (20 * time.Millisecond))
				}
			}

			// Wait for final debounce period
			time.Sleep(informerDebounceDelay + (20 * time.Millisecond))

			// Cleanup
			close(periodicQuit)

			// Verify results
			assert.Equal(t, tt.expectedCallCount, mockProv.count())
			logMessages := logBuffer.String()
			if len(tt.expectedBatchCounts) == 0 {
				assert.NotContains(t, logMessages, "CSPFK031I", "Batch log should not be present")
			} else {
				for _, expectedBatchCount := range tt.expectedBatchCounts {
					expectedLog := fmt.Sprintf(messages.CSPFK031I, expectedBatchCount)
					assert.Contains(t, logMessages, expectedLog, "Batch count log should be present")
				}
			}
		})
	}
}
