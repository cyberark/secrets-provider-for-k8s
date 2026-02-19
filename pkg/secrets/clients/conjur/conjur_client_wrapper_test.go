package conjur

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsV2NotAvailableError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "version too old error",
			err:      fmt.Errorf("not supported in Conjur versions older than 1.24.0"),
			expected: true,
		},
		{
			name:     "404 not found",
			err:      fmt.Errorf("404 endpoint not found"),
			expected: true,
		},
		{
			name:     "error with 'not supported in' phrase",
			err:      fmt.Errorf("Batch Retrieve Secrets API is not supported in this version"),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isV2NotAvailableError(tc.err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeVariableIdForV2(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full identifier with account and variable prefix",
			input:    "my-account:variable:secrets/password",
			expected: "secrets/password",
		},
		{
			name:     "full identifier with nested path",
			input:    "my-account:variable:app/db/credentials/password",
			expected: "app/db/credentials/password",
		},
		{
			name:     "already normalized path",
			input:    "secrets/test_secret",
			expected: "secrets/test_secret",
		},
		{
			name:     "simple variable name",
			input:    "password",
			expected: "password",
		},
		{
			name:     "variable with spaces",
			input:    "my-account:variable:secrets/var with spaces",
			expected: "secrets/var with spaces",
		},
		{
			name:     "variable with special characters",
			input:    "my-account:variable:secrets/var+with+pluses",
			expected: "secrets/var+with+pluses",
		},
		{
			name:     "variable with colon in path",
			input:    "my-account:variable:secrets/key:value",
			expected: "secrets/key:value",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeVariableIdForV2(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
