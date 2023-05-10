package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var (
	testenv        env.Environment
	KubeconfigFile string
	k8sClient      klient.Client
)

func TestMain(m *testing.M) {
	testenv = env.New()
	path := conf.ResolveKubeConfigFile()
	cfg := envconf.NewWithKubeConfig(path)
	testenv = env.NewWithConfig(cfg)

	testenv.Setup(
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println("Setup")
			k8sClient = cfg.Client()

			cmd1 := exec.Command("../bin/build")
			out, err := cmd1.CombinedOutput()
			if err != nil {
				fmt.Printf("Failed to execute command. %v, %s", err, out)
			}

			cmd2 := exec.Command("../bin/start", "--dev")
			out, err = cmd2.CombinedOutput()
			if err != nil {
				fmt.Printf("Failed to execute command. %v, %s", err, out)
			}

			// Get the Secrets Provider Pod
			var pods v1.PodList
			err = k8sClient.Resources("local-secrets-provider").List(context.TODO(), &pods)
			if err != nil {
				fmt.Print(err)
			}
			fmt.Printf("found %d pods in namespace local-secrets-provider", len(pods.Items))
			if len(pods.Items) == 0 {
				fmt.Printf("no pods in namespace local-secrets-provider")
			}

			// Wait for the Secrets Provider Pod to be ready
			pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pods.Items[0].Name}}
			err = wait.For(conditions.New(k8sClient.Resources(SecretsProviderNamespace)).PodReady(k8s.Object(&pod)), wait.WithTimeout(time.Minute*1))
			if err != nil {
				fmt.Print(err)
			}

			// Setup complete
			fmt.Println("Setup done")

			return ctx, nil
		},
	)
	testenv.Finish(
		envfuncs.DeleteNamespace("local-secrets-provider"),
		envfuncs.DeleteNamespace("local-conjur"),
	)
	testenv.BeforeEachTest(
		func(ctx context.Context, _ *envconf.Config, t *testing.T) (context.Context, error) {

			return ctx, nil
		},
	)
	testenv.AfterEachTest(
		func(ctx context.Context, _ *envconf.Config, t *testing.T) (context.Context, error) {
			envfuncs.DeleteNamespace("local-secrets-provider")

			return ctx, nil
		},
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}
