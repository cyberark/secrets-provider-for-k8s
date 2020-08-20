package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLimitedBackOff(t *testing.T) {
	Convey("Subject: Using limited backoff", t, func() {
		const (
			interval   = time.Second
			retryLimit = 3
		)

		Convey("Given a new limited backoff", func() {
			backOff := NewLimitedBackOff(interval, retryLimit)

			testLimitedBackOff(backOff, retryLimit, interval)
		})

		Convey("Given an exhausted limited backoff", func() {
			backOff := NewLimitedBackOff(interval, retryLimit)
			callMultipleNextBackOffs(backOff, retryLimit)

			Convey("When calling Reset", func() {
				backOff.Reset()

				assertRetryCount(backOff, 0)

				testLimitedBackOff(backOff, retryLimit, interval)
			})
		})
	})
}

func testLimitedBackOff(backOff *limitedBackOff, retryLimit int, interval time.Duration) {
	Convey("When calling NextBackOff until retry limit is reached", func() {
		results := callMultipleNextBackOffs(backOff, retryLimit)
		assertResultsEqualExpected(interval, results)
		assertRetryCount(backOff, retryLimit)

		const retryBeyondLimit = 10
		Convey(fmt.Sprint("When calling NextBackOff ", retryBeyondLimit, " times beyond limit"), func() {
			results := callMultipleNextBackOffs(backOff, retryBeyondLimit)
			assertResultsEqualExpected(backoff.Stop, results)
			assertRetryCount(backOff, retryLimit)
		})
	})
}

func callMultipleNextBackOffs(limitedBackOff *limitedBackOff, count int) []time.Duration {
	results := make([]time.Duration, count)
	for i := 0; i < count; i++ {
		results[i] = limitedBackOff.NextBackOff()
	}
	return results
}

func assertResultsEqualExpected(expected time.Duration, results []time.Duration) {
	Convey(fmt.Sprint("All backoff durations should equal ", expected), func() {
		for _, result := range results {
			So(result, ShouldEqual, expected)
		}
	})
}

func assertRetryCount(backOff *limitedBackOff, expected int) {
	Convey(fmt.Sprint("The RetryCount equals ", expected), func() {
		So(backOff.RetryCount(), ShouldEqual, expected)
	})
}
