package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestSecretsProvidedK8s(t *testing.T) {
	f := features.New("K8s secrets provided").
		Assess("test secret set in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "TEST_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "supersecret")

			return ctx
		}).
		Assess("umlaut secret set in pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"printenv", "|", "grep", "VARIABLE_WITH_UMLAUT_SECRET"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			assert.Contains(t, stdout.String(), "ÄäÖöÜü")

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestSecretsProvidedP2F(t *testing.T) {
	f := features.New("P2F secrets provided").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ReloadWithTemplate(cfg.Client(), P2fTemplate)
			return ctx
		}).
		Assess("secrets stored in file", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			command := []string{"cat", "opt/secrets/conjur/group1.yaml"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			yamlContent := "\"url\": \"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n\"username\": \"some-user\"\n\"password\": \"7H1SiSmYp@5Sw0rd\"\n\"encoded\": \"secret-value\""

			assert.Contains(t, stdout.String(), yamlContent)

			return ctx
		})

	testenv.Test(t, f.Feature())
}

func TestSecretsProvidedP2FRotation(t *testing.T) {
	f := features.New("P2F secrets provided and rotated").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ReloadWithTemplate(cfg.Client(), P2fRotationTemplate)
			return ctx
		}).
		Assess("secrets rotated in file", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			var stdout, stderr bytes.Buffer
			command := []string{"cat", "opt/secrets/conjur/group1.yaml"}

			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			yamlContent := "\"url\": \"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n\"username\": \"some-user\"\n\"password\": \"7H1SiSmYp@5Sw0rd\"\n\"test\": \"supersecret\"\n\"encoded\": \"secret-value\""

			assert.Contains(t, stdout.String(), yamlContent)

			SetConjurSecret(cfg.Client(), "secrets/encoded", base64.StdEncoding.EncodeToString([]byte("rotated-secret-value")))
			time.Sleep(15 * time.Second)
			RunCommandInSecretsProviderPod(cfg.Client(), command, &stdout, &stderr)

			yamlContent = "\"url\": \"postgresql://test-app-backend.app-test.svc.cluster.local:5432\"\n\"username\": \"some-user\"\n\"password\": \"7H1SiSmYp@5Sw0rd\"\n\"test\": \"supersecret\"\n\"encoded\": \"rotated-secret-value\""
			assert.Contains(t, stdout.String(), yamlContent)

			return ctx
		})

	testenv.Test(t, f.Feature())
}
