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

func badProvider() error {
	return errors.New("Failed to Provide")
}

func goodProvider() error {
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
