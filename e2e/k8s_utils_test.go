//go:build e2e
// +build e2e

package e2e

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTransientExecError(t *testing.T) {
	testCases := []struct {
		name           string
		execError      error
		stderrText     string
		expectedResult bool
	}{
		{
			name:           "transient container not found",
			execError:      errors.New("unable to upgrade connection: container not found (\"test-app\")"),
			stderrText:     "",
			expectedResult: true,
		},
		{
			name:           "transient timeout in stderr",
			execError:      errors.New("exec failed"),
			stderrText:     "dial tcp: i/o timeout",
			expectedResult: true,
		},
		{
			name:           "non transient permission error",
			execError:      errors.New("forbidden: user cannot exec into pod"),
			stderrText:     "",
			expectedResult: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actualResult := isTransientExecError(testCase.execError, testCase.stderrText)
			assert.Equal(t, testCase.expectedResult, actualResult)
		})
	}
}
