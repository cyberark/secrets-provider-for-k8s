package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimLeadingPathSegments(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		trim     int
		expected string
	}{
		{
			name:     "trim zero segments",
			path:     "one/two/three",
			trim:     0,
			expected: "one/two/three",
		},
		{
			name:     "trim one segment",
			path:     "one/two/three",
			trim:     1,
			expected: "two/three",
		},
		{
			name:     "trim two segments",
			path:     "one/two/three",
			trim:     2,
			expected: "three",
		},
		{
			name:     "trim many segments keeps path unchanged",
			path:     "one/two/three",
			trim:     4,
			expected: "one/two/three",
		},
		{
			name:     "trim exact number of segments keeps path unchanged",
			path:     "one/two/three",
			trim:     3,
			expected: "one/two/three",
		},
		{
			name:     "trim with negative segments keeps path unchanged",
			path:     "one/two/three",
			trim:     -1,
			expected: "one/two/three",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TrimLeadingPathSegments(tc.path, tc.trim)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseTrimPathSegments(t *testing.T) {
	testCases := []struct {
		name        string
		input       interface{}
		expected    int
		expectErr   bool
		errContains string
	}{
		{
			name:      "parses string value",
			input:     "2",
			expected:  2,
			expectErr: false,
		},
		{
			name:      "trims whitespace before parse",
			input:     "  3 ",
			expected:  3,
			expectErr: false,
		},
		{
			name:      "parses numeric integer",
			input:     4,
			expected:  4,
			expectErr: false,
		},
		{
			name:        "returns error for negative values",
			input:       -2,
			expectErr:   true,
			errContains: "expected a non-negative integer",
		},
		{
			name:        "returns error for non-numeric values",
			input:       "abc",
			expectErr:   true,
			errContains: "invalid syntax",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseTrimPathSegments(tc.input)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
