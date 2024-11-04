package pushtofile

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"slices"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur"
)

// Secret describes how Conjur secrets are represented in the Push-to-File context.
type Secret struct {
	Alias string
	Value string
}

// FetchSecretsForGroups fetches the secrets for all the groups and returns
// map of [group name] to [a slice of secrets for the group]. Callers of this
// function should decorate any errors with messages.CSPFK052E
func FetchSecretsForGroups(
	depRetrieveSecrets conjur.RetrieveSecretsFunc,
	secretGroups []*SecretGroup,
	traceContext context.Context,
) (map[string][]*Secret, error) {
	var err error
	secretsByGroup := map[string][]*Secret{}

	secretPaths := getAllPaths(secretGroups)
	secretValueById, err := depRetrieveSecrets(secretPaths, traceContext)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Clear Conjur secret values from memory
		for i := range secretValueById {
			for b := range secretValueById[i] {
				secretValueById[i][b] = 0
			}
		}
	}()

	for _, group := range secretGroups {
		for _, spec := range group.SecretSpecs {
			paths := []string{spec.Path}
			// If the path is "*", then we should populate all the fetched secrets
			if spec.Path == "*" {
				paths = []string{}
				for path := range secretValueById {
					paths = append(paths, path)
				}

				// In Fetch All mode, we need to sort the secrets alphabetically.
				// This is to ensure that the order of the secrets is deterministic, which
				// is important both for testing and to avoid unnecessary updates to the
				// secret files and other hard-to-debug issues.
				slices.Sort(paths)
			}

			for _, path := range paths {
				secret, err := getSecretValueByID(secretValueById, spec, path)
				// This error will occur when using Fetch All together with another group that has a secret spec
				// with a path that is not present in the fetched secrets. In this case, we should imitate the behavior
				// of a missing secret in non-Fetch All mode - i.e., return an error. This will allow the caller to
				// decide whether to leave the secret files as is or to delete them (if sanitize is enabled).
				if err != nil {
					return nil, fmt.Errorf(messages.CSPFK068E, path, group.Name)
				}
				secretsByGroup[group.Name] = append(secretsByGroup[group.Name], secret)
			}
		}
	}

	return secretsByGroup, err
}

func getSecretValueByID(secretValuesByID map[string][]byte, spec SecretSpec, path string) (*Secret, error) {
	alias := spec.Alias
	if alias == "" || alias == "*" {
		alias = path
	}

	// Get the secret value from the map
	sValue, ok := secretValuesByID[path]
	if !ok {
		err := fmt.Errorf(
			"secret with alias %q not present in fetched secrets",
			alias,
		)
		return nil, err
	}

	// Decode the secret value if it's base64 encoded
	sValue = decodeIfNeeded(spec, alias, sValue)

	secret := &Secret{
		Alias: alias,
		Value: string(sValue),
	}
	return secret, nil
}

// Decodes a secret from Base64 if the SecretSpec specifies that it is encoded.
// If the secret is not encoded, the original secret value is returned.
func decodeIfNeeded(spec SecretSpec, alias string, sValue []byte) []byte {
	if spec.ContentType != "base64" {
		return sValue
	}

	decodedSecretValue := make([]byte, base64.StdEncoding.DecodedLen(len(sValue)))
	_, err := base64.StdEncoding.Decode(decodedSecretValue, sValue)
	decodedSecretValue = bytes.Trim(decodedSecretValue, "\x00")
	if err != nil {
		// Log the error as a warning but still provide the original secret value
		log.Warn(messages.CSPFK064E, alias, spec.ContentType, err.Error())
	} else {
		return decodedSecretValue
	}

	return sValue
}

// secretPathSet is a mathematical set of secret paths. The values of the
// underlying map use an empty struct, since no data is required.
type secretPathSet map[string]struct{}

func (s secretPathSet) Add(path string) {
	s[path] = struct{}{}
}

func getAllPaths(secretGroups []*SecretGroup) []string {
	// Create a mathematical set of all secret paths
	pathSet := secretPathSet{}
	for _, group := range secretGroups {
		for _, spec := range group.SecretSpecs {
			// If the path is "*", then we should fetch all secrets
			if spec.Path == "*" {
				return []string{"*"}
			}

			pathSet.Add(spec.Path)
		}
	}
	// Convert the set of secret paths to a slice of secret paths
	var paths []string
	for path := range pathSet {
		paths = append(paths, path)
	}
	return paths
}
