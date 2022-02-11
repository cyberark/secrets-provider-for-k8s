package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	logger "github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/stretchr/testify/assert"
)

const (
	providerDelayMsecs      = 5
	largeProviderDelayMsecs = 50
)

// call count state, so that test cases don't interfere with one another.
type mockProvider struct {
	calledCount          int
	callLatencyMsecs     time.Duration
	injectFailure        bool
	failOnCountN         int
	injectInitialFailure bool
	failUntilCountN      int
}

func (m *mockProvider) provide() error {
	m.calledCount++
	switch {
	case m.injectFailure && (m.calledCount >= m.failOnCountN):
		return errors.New("Failed to Provide")
	case m.injectInitialFailure && (m.calledCount < m.failUntilCountN):
		return errors.New("Failed to Provide")
	case m.callLatencyMsecs > 0:
		time.Sleep(m.callLatencyMsecs * time.Millisecond)
	}
	return nil
}

func (m *mockProvider) count() int {
	return m.calledCount
}

func goodProvider() *mockProvider {
	return &mockProvider{}
}

func badProvider() *mockProvider {
	return &mockProvider{injectFailure: true, failOnCountN: 1}
}

func eventualProvider(failUntilCountN int) *mockProvider {
	return &mockProvider{injectInitialFailure: true, failUntilCountN: failUntilCountN}
}

func slowProvider(latencyMsecs time.Duration) *mockProvider {
	return &mockProvider{callLatencyMsecs: latencyMsecs}
}
func goodAtFirstThenBadProvider(failOnCountN int) *mockProvider {
	return &mockProvider{injectFailure: true, failOnCountN: failOnCountN}
}

func TestRetryableSecretProvider(t *testing.T) {
	TestCases := []struct {
		description string
		interval    time.Duration
		limit       int
		provider    *mockProvider
		assertOn    string
	}{
		{
			description: "the ProviderFunc can return within the retry limit",
			interval:    time.Duration(10) * time.Millisecond,
			limit:       3,
			provider:    eventualProvider(3),
			assertOn:    "success",
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
	}

	for _, tc := range TestCases {
		var logBuffer bytes.Buffer
		logger.InfoLogger = log.New(&logBuffer, "", 0)

		retryableProvider := RetryableSecretProvider(
			tc.interval, tc.limit, tc.provider.provide,
		)

		start := time.Now()
		err := retryableProvider()
		duration := time.Since(start)

		logMessages := logBuffer.String()
		if tc.assertOn == "count" {
			assert.NotNil(t, err)
			assert.Contains(t, logMessages, fmt.Sprintf("CSPFK010I Updating Kubernetes Secrets: %d retries out of %d", tc.limit, tc.limit))
		} else if tc.assertOn == "interval" {
			assert.NotNil(t, err)
			maxDuration := (time.Duration(tc.limit) * time.Millisecond * tc.interval) + (time.Duration(1) * time.Millisecond)
			assert.LessOrEqual(t, duration, maxDuration)
		} else if tc.assertOn == "success" {
			assert.NoError(t, err)
		}
	}
}

func TestSecretProvider(t *testing.T) {
	testCases := []struct {
		description   string
		mode          string
		interval      time.Duration
		testTime      time.Duration // total test time for all tests must be less than 3m
		expectedCount int
		provider      *mockProvider
		assertOn      string
	}{
		{
			description:   "init container, happy path",
			mode:          "init",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(25) * time.Millisecond,
			expectedCount: 1,
			provider:      goodProvider(),
			assertOn:      "success",
		},
		{
			description:   "sidecar container, happy path",
			mode:          "sidecar",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(65) * time.Millisecond,
			expectedCount: 7,
			provider:      goodProvider(),
			assertOn:      "success",
		},
		{
			description:   "sidecar with zero duration",
			mode:          "sidecar",
			interval:      time.Duration(0) * time.Millisecond,
			testTime:      time.Duration(25) * time.Millisecond,
			expectedCount: 1,
			provider:      goodProvider(),
			assertOn:      "success",
		},
		{
			description:   "application mode, happy path",
			mode:          "application",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(25) * time.Millisecond,
			expectedCount: 1,
			provider:      goodProvider(),
			assertOn:      "success",
		},
		{
			// The provider is slow, but still finishes within the interval time.
			// Note the ticker is started after the first provideSecrets call so
			// that delay is added to the duration
			description:   "sidecar with slow provider",
			mode:          "sidecar",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(65+providerDelayMsecs) * time.Millisecond,
			expectedCount: 7,
			provider:      slowProvider(providerDelayMsecs),
			assertOn:      "success",
		},
		{
			// In this test the provider takes longer to run than the
			// interval time. The Go timer will adjust the time interval due to the
			// slower receiver. Secrets Provider is expected to be called every largeProviderDelayMsecs
			// The first timer tick is delayed by duration so that is added to the test time.
			description:   "sidecar with duration less than fetch time",
			mode:          "sidecar",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(325+10) * time.Millisecond,
			expectedCount: 7,
			provider:      slowProvider(largeProviderDelayMsecs),
			assertOn:      "success",
		},
		{
			description:   "badProvider for init container",
			mode:          "init",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(25) * time.Millisecond,
			expectedCount: 1,
			provider:      badProvider(),
			assertOn:      "fail",
		},
		{
			description:   "badProvider for sidecar",
			mode:          "sidecar",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(25) * time.Millisecond,
			expectedCount: 1,
			provider:      badProvider(),
			assertOn:      "fail",
		},
		{
			description:   "goodAtFirstThenBadProvider for sidecar",
			mode:          "sidecar",
			interval:      time.Duration(10) * time.Millisecond,
			testTime:      time.Duration(35) * time.Millisecond,
			expectedCount: 1,
			provider:      goodAtFirstThenBadProvider(2),
			assertOn:      "fail",
		},
	}

	for _, tc := range testCases {
		var err error

		// Construct a secret provider function
		var providerQuit = make(chan struct{})
		provideSecrets := SecretProvider(
			tc.interval, tc.mode, tc.provider.provide, providerQuit,
		)

		testError := make(chan error)
		go func() {
			err := provideSecrets()
			testError <- err
		}()
		select {
		case err = <-testError:
			break
		case <-time.After(tc.testTime):
			providerQuit <- struct{}{}
			break
		}

		if err == nil && tc.provider.count() != tc.expectedCount {
			err = fmt.Errorf("%s: incorrect number of timer ticks, got %d expected %d",
				tc.description, tc.provider.count(), tc.expectedCount)
		}

		if tc.assertOn == "fail" {
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "Failed to Provide")
		} else if tc.assertOn == "success" {
			assert.NoError(t, err)
		}
	}
}
