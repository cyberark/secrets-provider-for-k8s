package main

import (
	"flag"
	"os"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/pushtofile"
)

func main() {
	var annotationFilePath string
	flag.StringVar(&annotationFilePath, "f", "./annotations", "path to annotation file")

	flag.Parse()

	// Parse annotations from downward API
	log.Info("Parse annotations from downward API")
	annotations, err := annotations.NewAnnotationsFromFile(annotationFilePath)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
		return
	}

	// Generate secret Groups
	log.Info("Generate secret Groups")
	secretsBasePath, _ := os.Getwd()
	secretGroups, errs := pushtofile.NewSecretGroups(secretsBasePath, annotations)
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
	log.Info("Fetching secrets")

	secretsByGroup, err := fetchSecretsForGroups(
		func(variableIDs []string) (map[string][]byte, error) {
			var res = map[string][]byte{}
			for _, id := range variableIDs {
				log.Info("Processing secret %s", id)
				res[id] = []byte("value-" + id)
			}

			return res, nil
		}, secretGroups)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
		return
	}

	// Write secrets to file
	exitStatus := 0
	for _, group := range secretGroups {
		log.Info("Process template for %s to %s\n", group.Name, group.FilePath)

		err := group.PushToFile(secretsByGroup[group.Name])
		if err != nil {
			log.Error(err.Error())
			exitStatus = 1
		}
	}
	os.Exit(exitStatus)
}

func fetchSecretsForGroups(
	retrieveConjurSecrets func(variableIDs []string) (map[string][]byte, error),
	secretGroups []*pushtofile.SecretGroup,
) (map[string][]*pushtofile.Secret, error) {
	// map[group name] => group secret vales

	// Gather secret paths
	var secretPaths []string
	var uniqueSecretPaths = map[string]struct{}{}
	for _, group := range secretGroups {
		log.Info("Processing group", group)
		specs := group.ResolvedSecretSpecs()
		for _, spec := range specs {
			if _, ok := uniqueSecretPaths[spec.Path]; ok {
				continue
			}

			uniqueSecretPaths[spec.Path] = struct{}{}
			secretPaths = append(secretPaths, spec.Path)
		}
	}

	// Get access token and fetch the secrets
	// TODO: Create authenticator responsible for populating access token
	// TODO: create better abstraction that hides authenticator and retry logic from the
	// rest of the code
	//
	secretValueByPath, err := retrieveConjurSecrets(secretPaths)
	if err != nil {
		return nil, err
	}

	// Gather secret values
	var secretsByGroup = map[string][]*pushtofile.Secret{}
	for _, group := range secretGroups {
		specs := group.ResolvedSecretSpecs()

		groupSecrets := make([]*pushtofile.Secret, len(specs))

		for i, spec := range specs {
			secretValue := secretValueByPath[spec.Path]
			groupSecrets[i] = &pushtofile.Secret{
				Alias: spec.Alias,
				Value: string(secretValue),
			}
		}
		secretsByGroup[group.Name] = groupSecrets
	}

	return secretsByGroup, nil
}
