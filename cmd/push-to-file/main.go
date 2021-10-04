package main

import (
	"flag"
	"os"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage/mocks"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/push_to_file"
)

func main() {
	var annotationFilePath string
	flag.StringVar(&annotationFilePath, "f", "", "path to annotation file")

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
	log.Info("Fetching secrets")

	atoken := mocks.MockAccessToken{}
	secretsByGroup, err := fetchSecretsForGroups(
		atoken, nil, secretGroups)
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
	accessToken access_token.AccessToken,
	retrieveConjurSecrets conjur.RetrieveConjurSecretsFunc,
	secretGroups []*push_to_file.SecretGroup,
) (map[string][]*push_to_file.Secret, error) {
// map[group name] => group secret vales

	// Gather secret paths
	var secretPaths []string
	var uniqueSecretPaths = map[string]struct{}{}
	for _, group := range secretGroups {
		specs := group.ResolvedSecretSpecs()
		for _, spec := range specs {
			if _, ok := uniqueSecretPaths[spec.Path]; !ok {
				uniqueSecretPaths[spec.Path] = struct{}{}
				continue
			}

			secretPaths = append(secretPaths, spec.Path)
		}
	}

	// Get access token and fetch the secrets
	// TODO: Create authenticator responsible for populating access token
	// TODO: create better abstraction that hides authenticator and retry logic from the
	// rest of the code
	//
	// TODO: provide a single interface that this method takes as input
	// the interface should hide access tokens etc.
	accessTokenData, err := accessToken.Read()
	if err != nil {
		return nil, err
	}
	secretValueByPath, err := retrieveConjurSecrets(accessTokenData, secretPaths)
	if err != nil {
		return nil, err
	}

	// Gather secret values
	var secretsByGroup = map[string][]*push_to_file.Secret{}
	for _, group := range secretGroups {
		specs := group.ResolvedSecretSpecs()

		groupSecrets := make([]*push_to_file.Secret, len(specs))

		for i, spec := range specs {
			secretValue := secretValueByPath[spec.Path]
			groupSecrets[i] = &push_to_file.Secret{
				Alias: spec.Alias,
				Value: string(secretValue),
			}
		}
		secretsByGroup[group.Name] = groupSecrets
	}

	return secretsByGroup, nil
}
