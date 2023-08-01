//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			fmt.Println("Verifying the secrets provider pod is ready")
			var pods v1.PodList
			FetchPodsFromNamespace(k8sClient, SecretsProviderNamespace, &pods)
			pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pods.Items[0].Name}}
			err := wait.For(conditions.New(k8sClient.Resources(SecretsProviderNamespace)).PodReady(k8s.Object(&pod)), wait.WithTimeout(time.Minute*1))
			if err != nil {
				fmt.Print(err)
			}
			return ctx, nil
		},
	)
	testenv.Finish(
	// TODO - Delete the namespaces after all tests run
	// For dev purposes it is helpful to leave the configured cluster up

	// envfuncs.DeleteNamespace(SecretsProviderNamespace),
	// envfuncs.DeleteNamespace(ConjurNamespace),
	)
	testenv.AfterEachTest(
		func(ctx context.Context, _ *envconf.Config, t *testing.T) (context.Context, error) {
			// TODO - Delete the secrets provider namespace after each test so we can reconfigure as needed

			// envfuncs.DeleteNamespace(SecretsProviderNamespace),
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}
