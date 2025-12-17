//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestAuthnAzure(t *testing.T) {
	f := features.New("authn-azure").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Setenv("CONJUR_AUTHN_LOGIN", "host/conjur/authn-azure/"+AuthenticatorId()+"/apps/azureVM")
			t.Setenv("CONJUR_AUTHN_TYPE", "azure")
			t.Setenv("CONJUR_AUTHN_URL", ConjurApplianceUrl()+"/authn-azure/"+AuthenticatorId())

			err := ReloadWithTemplate(cfg.Client(), K8sTemplate)
			require.Nil(t, err)

			return ctx
		}).
		Assess("ssh key set correctly in pod", assessSSHKey).
		Assess("json object secret set correctly in pod", assessJSONObject).
		Assess("variables with spaces secret set correctly in pod", assessVariablesWithSpaces).
		Assess("variables with pluses secret set correctly in pod", assessVariablesWithPluses).
		Assess("umlaut secret set correctly in pod", assessUmlaut).
		Assess("variables with base64 decoding secret set correctly in pod", assessBase64Decoding).
		Assess("non conjur keys stay intact secret set correctly in pod", assessNonConjurKeys).
		Assess("fetch all secrets provided", assessFetchAllSecrets).
		Assess("fetch all base64 secrets provided", assessFetchAllBase64)

	testenv.Test(t, f.Feature())
}
