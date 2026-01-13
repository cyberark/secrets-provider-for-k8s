package utils

import "strconv"

// IsTrue returns true if the string value represents a boolean true
func IsTrue(val string) bool {
	boolVal, err := strconv.ParseBool(val)
	return err == nil && boolVal
}
