//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv   env.Environment
	k8sClient klient.Client
)

func TestMain(m *testing.M) {
	testenv = env.New()
	path := conf.ResolveKubeConfigFile()
	cfg := envconf.NewWithKubeConfig(path)
	testenv = env.NewWithConfig(cfg)
	k8sClient = cfg.Client()

	testenv.Setup(
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			spPod, err := FetchPodWithLabelSelector(k8sClient, SecretsProviderNamespace(), SPLabelSelector)
			if err != nil {
				return ctx, err
			}

			err = wait.For(conditions.New(k8sClient.Resources(SecretsProviderNamespace())).PodReady(k8s.Object(&spPod)), wait.WithTimeout(time.Minute*2))
			if err != nil {
				return ctx, err
			}

			return ctx, nil
		},
	)

	testenv.AfterEachTest(
		func(ctx context.Context, _ *envconf.Config, t *testing.T) (context.Context, error) {
			DumpFailureDiagnosticsOnFailure(t)

			return ctx, nil
		},
	)

	testenv.Finish(
	// Teardown is orchestrated by individual tests so we may not need this
	)

	os.Exit(testenv.Run(m))
}
