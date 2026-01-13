package utils

import "testing"

func TestIsTrue(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid true values
		{name: "string 'true'", input: "true", expected: true},
		{name: "string 'True'", input: "True", expected: true},
		{name: "string 'TRUE'", input: "TRUE", expected: true},
		{name: "string '1'", input: "1", expected: true},
		{name: "string 't'", input: "t", expected: true},
		{name: "string 'T'", input: "T", expected: true},

		// Valid false values
		{name: "string 'false'", input: "false", expected: false},
		{name: "string 'False'", input: "False", expected: false},
		{name: "string 'FALSE'", input: "FALSE", expected: false},
		{name: "string '0'", input: "0", expected: false},
		{name: "string 'f'", input: "f", expected: false},
		{name: "string 'F'", input: "F", expected: false},

		// Invalid values (should return false)
		{name: "empty string", input: "", expected: false},
		{name: "random string", input: "yes", expected: false},
		{name: "random string 'no'", input: "no", expected: false},
		{name: "number '2'", input: "2", expected: false},
		{name: "whitespace", input: " ", expected: false},
		{name: "string with spaces", input: " true ", expected: false},
		{name: "special characters", input: "!@#", expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsTrue(tc.input)
			if result != tc.expected {
				t.Errorf("IsTrue(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}
