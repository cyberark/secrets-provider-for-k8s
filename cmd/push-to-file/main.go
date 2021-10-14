package main

import (
	"flag"
	"os"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
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
	secretGroups, err := push_to_file.NewSecretGroups(annotations)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
		return
	}

	// Fetch secrets MOCK!
	// TODO: Should we make sure any secret id is only fetched once ?
	// TODO: Should we zeroize the secrets map when we're done ?
	// TODO: secret fetching should be concurrent and where possible parallel
	log.Info("Fetching secrets")

	secretsByGroup, err := push_to_file.FetchSecretsForGroups(
		func(variableIDs []string) (map[string][]byte, error) {
			var res = map[string][]byte{}
			for _, id := range variableIDs {
				res[id] = []byte("value-" + id)
			}

			return res, nil
		},
		secretGroups,
	)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
		return
	}

	// Write secrets to file
	exitStatus := 0
	for _, group := range secretGroups {
		log.Info("Process template for %s to %s\n", group.Name, group.FilePath)

		err := group.PushToFile("", secretsByGroup[group.Name])
		if err != nil {
			log.Error(err.Error())
			exitStatus = 1
		}
	}
	os.Exit(exitStatus)
}

