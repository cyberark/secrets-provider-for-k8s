//go:build e2e
// +build e2e

package e2e

import "os"

func getenvDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
