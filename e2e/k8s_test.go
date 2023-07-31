//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestSecretsProvidedK8s(t *testing.T) {
	f := features.New("secrets provided (K8s secrets mode)").
		// Replaces TEST_ID_1.5_providing_variables_with_german_umlaut_successfully
		Assess("umlaut secret set correctly in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_UMLAUT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "ÄäÖöÜü")

			return ctx
		})

	testenv.Test(t, f.Feature())
}
