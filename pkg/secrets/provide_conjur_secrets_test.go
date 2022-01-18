package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	logger "github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
	"github.com/stretchr/testify/assert"
)

const (
	providerDelayTime = 500
)
var providerCount = 0

func badProvider() error {
	return errors.New("Failed to Provide")
}

func goodAtFirstThenBadProvider() error {
	providerCount++
	if providerCount > 2 {
		return errors.New("Failed to Provide")
	}
	return nil
}

func goodProvider() error {
	providerCount++
	return nil
}

func slowProvider() error {
	providerCount++
	time.Sleep(providerDelayTime * time.Millisecond)
	return nil
}

func eventualProvider(successAfterNtries int) func() error {
	limitedBackOff := utils.NewLimitedBackOff(
		time.Duration(1)*time.Millisecond,
		3,
	)

	return func() error {
		err := backoff.Retry(func() error {
			if limitedBackOff.RetryCount() == successAfterNtries {
				return goodProvider()
			}

			return errors.New("throw error")
		}, limitedBackOff)

		return err
	}
}

func TestRetryableSecretProvider(t *testing.T) {
	TestCases := []struct {
		description string
		interval    time.Duration
		limit       int
		provider    ProviderFunc
		assertOn    string
	}{
		{
			description: "the ProviderFunc can return within the retry limit",
			interval:    time.Duration(10) * time.Millisecond,
			limit:       3,
			provider:    eventualProvider(2),
			assertOn:    "success",
		},
		{
			description: "the ProviderFunc is retried the proper amount of times",
			interval:    time.Duration(10) * time.Millisecond,
			limit:       2,
			provider:    badProvider,
			assertOn:    "count",
		},
		{
			description: "the ProviderFunc is only retried after the proper duration",
			interval:    time.Duration(20) * time.Millisecond,
			limit:       2,
			provider:    badProvider,
			assertOn:    "interval",
		},
	}

	for _, tc := range TestCases {
		var logBuffer bytes.Buffer
		logger.InfoLogger = log.New(&logBuffer, "", 0)

		retryableProvider := RetryableSecretProvider(
			tc.interval, tc.limit, tc.provider,
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

func TestPeriodicSecretProvider(t *testing.T) {
	TestCases := []struct {
		description string
		mode        string
		interval    time.Duration
		testTime    time.Duration // total test time for all tests must be less than 3m
		expectedCount         int
		provider    ProviderFunc
		assertOn    string
	}{
		{
			description: "init",
			mode:        "init",
			interval:    time.Duration(500) * time.Millisecond,
			testTime:    time.Duration(1000) * time.Millisecond,
			expectedCount:  1,
			provider:    goodProvider,
			assertOn:    "success",
		},
		{
			description: "sidecar",
			mode:        "sidecar",
			interval:    time.Duration(500) * time.Millisecond,
			testTime:    time.Duration(3250) * time.Millisecond,
			expectedCount:  7,
			provider:    goodProvider,
			assertOn:    "success",
		},
		{
			description: "sidecar with zero duration",
			mode:        "sidecar",
			interval:    time.Duration(0) * time.Millisecond,
			testTime:    time.Duration(3000) * time.Millisecond,
			expectedCount:  1,
			provider:    goodProvider,
			assertOn:    "success",
		},
		{
			description: "application",
			mode:        "application",
			interval:    time.Duration(500) * time.Millisecond,
			testTime:    time.Duration(3000) * time.Millisecond,
			expectedCount:  1,
			provider:    goodProvider,
			assertOn:    "success",
		},
		// This test is inconsistent, 3-4 ticks
		/*{
			description: "sidecar with slow provider",
			mode:        "sidecar",
			interval:    time.Duration(1000) * time.Millisecond,
			testTime:    time.Duration(3500) * time.Millisecond,
			expectedCount:  3000/1000,
			provider:    slowProvider,
			assertOn:    "success",
		},*/
		// This test is inconsistent, 11-13 ticks
		/*{
			// In this test the provider takes longer to run than the
			// interval time. The Go ticker will adjust the time interval due to the
			// slower receiver.
			description: "sidecar with duration less than fetch time",
			mode:        "sidecar",
			interval:    time.Duration(300) * time.Millisecond,
			testTime:    time.Duration(3000) * time.Millisecond,
			expectedCount:  3000/providerDelayTime+1,
			provider:    slowProvider,
			assertOn:    "success",
		},*/
		{
			description: "badProvider for init container",
			mode:        "init",
			interval:    time.Duration(200) * time.Millisecond,
			testTime:    time.Duration(500) * time.Millisecond,
			expectedCount:  1,
			provider:    badProvider,
			assertOn:    "fail",
		},
		{
			description: "badProvider for sidecar",
			mode:        "sidecar",
			interval:    time.Duration(200) * time.Millisecond,
			testTime:    time.Duration(500) * time.Millisecond,
			expectedCount:  1,
			provider:    badProvider,
			assertOn:    "fail",
		},
		{
			description: "goodAtFirstThenBadProvider for sidecar",
			mode:        "sidecar",
			interval:    time.Duration(200) * time.Millisecond,
			testTime:    time.Duration(500) * time.Millisecond,
			expectedCount:  1,
			provider:    goodAtFirstThenBadProvider,
			assertOn:    "fail",
		},
	}

	for _, tc := range TestCases {
		//var logBuffer bytes.Buffer
		var err error
		//logger.InfoLogger = log.New(&logBuffer, "", 0)

		periodicProvider := PeriodicSecretProvider(
			tc.interval, tc.mode, tc.provider,
		)
		providerCount = 0
		go periodicProvider()
		select {
		case err = <- SecretProviderError:
		case <-time.After(tc.testTime):
			ProviderDone <- true
			time.Sleep(500 * time.Millisecond)
		}

		if err == nil && providerCount != tc.expectedCount {
			err = fmt.Errorf("%s: incorrect number of timer ticks, got %d expected %d",
				tc.description, providerCount, tc.expectedCount)
		}

		if tc.assertOn == "fail" {
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "Failed to Provide")
		} else if tc.assertOn == "success" {
			assert.NoError(t, err)
		}
	}
}
