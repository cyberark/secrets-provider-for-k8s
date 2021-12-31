package pushtofile

import (
	"context"
	"fmt"
	"syscall"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	sharedprocnamespace "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/shared_proc_namespace"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/trace"
	"go.opentelemetry.io/otel"
)

type fileProvider struct {
	retrieveSecretsFunc  conjur.RetrieveSecretsFunc
	secretGroups         []*SecretGroup
	restartAppSignal     syscall.Signal
	fileLockDuringUpdate bool
	traceContext         context.Context
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
	annotations map[string]string,
	restartAppSignal syscall.Signal,
	fileLockDuringUpdate bool,
) (*fileProvider, []error) {

	secretGroups, err := NewSecretGroups(secretsBasePath, templatesBasePath, annotations)
	if err != nil {
		return nil, err
	}

	fmt.Printf("***TEMP*** NewProvider, restartAppSignal = %d\n", restartAppSignal)
	return &fileProvider{
		retrieveSecretsFunc:  retrieveSecretsFunc,
		secretGroups:         secretGroups,
		restartAppSignal:     restartAppSignal,
		fileLockDuringUpdate: fileLockDuringUpdate,
		traceContext:         nil,
	}, nil
}

// Provide implements a ProviderFunc to retrieve and push secrets to the filesystem.
func (p fileProvider) Provide() error {
	return provideWithDeps(
		p.traceContext,
		p.secretGroups,
		p.restartAppSignal,
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
	restartAppSignal syscall.Signal,
	depFuncs fileProviderDepFuncs,
) error {
	fmt.Printf("***TEMP*** provideWithDeps, restartAppSignal = %d\n", restartAppSignal)
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

	spanCtx, span = tr.Start(traceContext, "Write Secret Files")
	defer span.End()
	filesChanged := false
	for _, group := range groups {
		_, childSpan := tr.Start(spanCtx, "Write Secret Files for group")
		defer childSpan.End()
		changed, err := group.pushToFileWithDeps(
			depFuncs.depOpenWriteCloser,
			depFuncs.depPushToWriter,
			secretsByGroup[group.Name],
		)
		if err != nil {
			childSpan.RecordErrorAndSetStatus(err)
			span.RecordErrorAndSetStatus(err)
			return err
		}
		if changed {
			filesChanged = true
		}
	}

	if filesChanged {
		log.Info(messages.CSPFK015I)
		fmt.Printf("***TEMP*** calling RestartApplication with restartAppSignal = %d\n", restartAppSignal)
		sharedprocnamespace.RestartApplication(restartAppSignal)
	} else {
		log.Info(messages.CSPFK018I)
	}
	fmt.Printf("==============================================\n\n\n\n")
	return nil
}
