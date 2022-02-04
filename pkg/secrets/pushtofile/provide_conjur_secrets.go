package pushtofile

import (
	"context"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/gofrs/flock"
	"go.opentelemetry.io/otel"
)

type fileProvider struct {
	retrieveSecretsFunc conjur.RetrieveSecretsFunc
	secretGroups        []*SecretGroup
	traceContext        context.Context
}

type fileProviderDepFuncs struct {
	retrieveSecretsFunc conjur.RetrieveSecretsFunc
	depOpenWriteCloser  openWriteCloserFunc
	depPushToWriter     pushToWriterFunc
}

// NewProvider creates a new provider for Push-to-File mode.
func NewProvider(
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
		traceContext:        nil,
	}, nil
}

// Provide implements a ProviderFunc to retrieve and push secrets to the filesystem.
func (p fileProvider) Provide() error {
	return provideWithDeps(
		p.traceContext,
		p.secretGroups,
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
	depFuncs fileProviderDepFuncs,
) error {
	// Use the global TracerProvider
	tr := trace.NewOtelTracer(otel.Tracer("secrets-provider"))
	spanCtx, span := tr.Start(traceContext, "Fetch Conjur Secrets")
	secretsByGroup, err := FetchSecretsForGroups(depFuncs.retrieveSecretsFunc, groups, spanCtx)
	if err != nil {
		span.RecordErrorAndSetStatus(err)
		span.End()
		return err
	}
	span.End()

	// Lock all files that will be written to
	_, span = tr.Start(traceContext, "Lock Files")
	fileLocks := map[string]*flock.Flock{}
	defer func() {
		_, span = tr.Start(traceContext, "Unlock Files")
		// Unlock all files that were locked
		for _, lock := range fileLocks {
			err = lock.Unlock()
			if err != nil {
				log.Error("Unable to unlock file", err)
				span.RecordErrorAndSetStatus(err)
			}
			log.Info("Unlocked file %s", lock.Path())
		}
		span.End()
	}()

	for _, group := range groups {
		filePath := group.FilePath
		// Check if we already locked this file
		if fileLocks[filePath] == nil {
			lock := flock.New(filePath)
			locked, err := lock.TryLock()
			if locked {
				log.Info("Locked file %s", filePath)
				fileLocks[filePath] = lock
			}
			if err != nil || !locked {
				log.Error("Failed to lock file", err)
				span.RecordErrorAndSetStatus(err)
			}
		}
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
