package pushtofile

import (
	"context"
	"os"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/config"
	"go.opentelemetry.io/otel"
)

type fileProvider struct {
	retrieveSecretsFunc conjur.RetrieveSecretsFunc
	secretGroups        []*SecretGroup
	traceContext        context.Context
	sanitizeEnabled     bool
}

type fileProviderDepFuncs struct {
	retrieveSecretsFunc conjur.RetrieveSecretsFunc
	depOpenWriteCloser  openWriteCloserFunc
	depPushToWriter     pushToWriterFunc
}

// NewProvider creates a new provider for Push-to-File mode.
func NewProvider(
	retrieveSecretsFunc conjur.RetrieveSecretsFunc,
	providerConfig *config.ProviderConfig) (*fileProvider, []error) {

	secretGroups, err := NewSecretGroups(
		providerConfig.SecretFileBasePath,
		providerConfig.TemplateFileBasePath,
		providerConfig.AnnotationsMap,
	)
	if err != nil {
		return nil, err
	}

	return &fileProvider{
		retrieveSecretsFunc: retrieveSecretsFunc,
		secretGroups:        secretGroups,
		traceContext:        nil,
		sanitizeEnabled:     providerConfig.SanitizeEnabled,
	}, nil
}

// Provide implements a ProviderFunc to retrieve and push secrets to the filesystem.
func (p fileProvider) Provide() error {
	return provideWithDeps(
		p.traceContext,
		p.secretGroups,
		p.sanitizeEnabled,
		fileProviderDepFuncs{
			retrieveSecretsFunc: p.retrieveSecretsFunc,
			depOpenWriteCloser:  openFileAsWriteCloser,
			depPushToWriter:     pushToWriter,
		},
	)
}

func (p *fileProvider) SetTraceContext(ctx context.Context) {
	p.traceContext = ctx
}

func provideWithDeps(
	traceContext context.Context,
	groups []*SecretGroup,
	sanitizeEnabled bool,
	depFuncs fileProviderDepFuncs,
) error {
	// Use the global TracerProvider
	tr := trace.NewOtelTracer(otel.Tracer("secrets-provider"))
	spanCtx, span := tr.Start(traceContext, "Fetch Conjur Secrets")
	secretsByGroup, err := FetchSecretsForGroups(depFuncs.retrieveSecretsFunc, groups, spanCtx)
	if err != nil {
		if sanitizeEnabled {
			// Delete secret files for variables that no longer exist or the user no longer has permissions to
			// TODO: Should we check the error message to see if it's a 404 or 403?
			for _, group := range groups {
				os.Remove(group.FilePath)
				log.Info(messages.CSPFK019I)
			}
		}

		span.RecordErrorAndSetStatus(err)
		span.End()
		return err
	}
	span.End()

	spanCtx, span = tr.Start(traceContext, "Write Secret Files")
	defer span.End()
	for _, group := range groups {
		_, childSpan := tr.Start(spanCtx, "Write Secret Files for group")
		defer childSpan.End()
		err := group.pushToFileWithDeps(
			depFuncs.depOpenWriteCloser,
			depFuncs.depPushToWriter,
			secretsByGroup[group.Name],
		)
		if err != nil {
			childSpan.RecordErrorAndSetStatus(err)
			span.RecordErrorAndSetStatus(err)
			return err
		}
	}

	log.Info(messages.CSPFK015I)
	return nil
}
