package main

import (
	"fmt"
	"os"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/push_to_file"
)

func main() {
	// Parse annotations from downward API
	log.Info("Parse annotations from downward API")
	annotations, err := annotations.NewAnnotationsFromFile("./testdata/annotations.txt")
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
		return
	}

	// Generate secret Groups
	log.Info("Generate secret Groups")
	secretGroups, errs := push_to_file.NewSecretGroups(annotations)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Error(err.Error())
		}
		os.Exit(1)
		return
	}

	// Fetch secrets MOCK!
	// TODO: Should we make sure any secret id is only fetched once ?
	// TODO: Should we zeroize the secrets map when we're done ?
	// TODO: secret fetching should be concurrent and where possible parallel
	groupSecrets := map[string][]*push_to_file.Secret{}
	for _, group := range secretGroups {
		log.Info("Fetch secrets for %s", group.Name)
		var secrets = make([]*push_to_file.Secret, len(group.SecretSpecs))

		for i, spec := range group.ResolvedSecretSpecs() {
			log.Info("Fetch %s", spec.Path)
			secrets[i] = &push_to_file.Secret{
				Alias: spec.Alias,
				Value: fmt.Sprintf("s-%s", spec.Alias),
			}
		}

		groupSecrets[group.Name] = secrets
	}

	// Write secrets to file
	exitStatus := 0
	for _, group := range secretGroups {
		log.Info("Process template for %s to %s\n", group.Name, group.FilePath)

		err := group.PushToFile(groupSecrets[group.Name])
		if err != nil {
			log.Error(err.Error())
			exitStatus = 1
		}
	}
	os.Exit(exitStatus)
}
