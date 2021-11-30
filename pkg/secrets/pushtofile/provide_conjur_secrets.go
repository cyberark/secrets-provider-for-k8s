package pushtofile

import (
	"context"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/trace"
	"go.opentelemetry.io/otel"
)

type fileProvider struct {
	retrieveSecretsFunc conjur.RetrieveSecretsFunc
	secretGroups        []*SecretGroup
	traceContext        context.Context
}

// NewProvider creates a new provider for Push-to-File mode.
func NewProvider(
	traceContext context.Context,
	retrieveSecretsFunc conjur.RetrieveSecretsFunc,
	secretsBasePath string,
	templatesBasePath string,
	annotations map[string]string) (*fileProvider, []error) {

	secretGroups, err := NewSecretGroups(secretsBasePath, templatesBasePath, annotations)
	if err != nil {
		return nil, err
	}

	return &fileProvider{
		retrieveSecretsFunc: retrieveSecretsFunc,
		secretGroups:        secretGroups,
		traceContext:        traceContext,
	}, nil
}

// Provide implements a ProviderFunc to retrieve and push secrets to the filesystem.
func (p fileProvider) Provide() error {
	return provideWithDeps(
		p.traceContext,
		p.secretGroups,
		p.retrieveSecretsFunc,
		openFileAsWriteCloser,
		pushToWriter,
	)
}

func provideWithDeps(
	traceContext context.Context,
	groups []*SecretGroup,
	retrieveSecretsFunc conjur.RetrieveSecretsFunc,
	depOpenWriteCloser openWriteCloserFunc,
	depPushToWriter pushToWriterFunc,
) error {
	// Use the global TracerProvider
	tr := trace.NewOtelTracer(otel.Tracer("secrets-provider"))
	_, span := tr.Start(traceContext, "Fetch Conjur Secrets")
	secretsByGroup, err := FetchSecretsForGroups(retrieveSecretsFunc, groups)
	defer span.End()
	if err != nil {
		span.RecordErrorAndSetStatus(err)
		return err
	}

	_, span = tr.Start(traceContext, "Write Secret Files")
	for _, group := range groups {
		err := group.pushToFileWithDeps(
			depOpenWriteCloser,
			depPushToWriter,
			secretsByGroup[group.Name],
		)
		if err != nil {
			span.RecordErrorAndSetStatus(err)
			return err
		}
	}

	log.Info(messages.CSPFK015I)
	return nil
}
