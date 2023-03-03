package pushtofile

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	maxConjurVarNameLen = 126
)

// SecretSpec specifies a secret to be retrieved from Conjur by defining
// its alias (i.e. the name of the secret from an application's perspective)
// and its variable path in Conjur.
type SecretSpec struct {
	Alias       string
	Path        string
	ContentType string
}

// MarshalYAML is a custom marshaller for SecretSpec.
func (t SecretSpec) MarshalYAML() (interface{}, error) {
	return map[string]string{t.Alias: t.Path, "ContentType": t.ContentType}, nil
}

const invalidSecretSpecErr = `expected a "string (path)" or "single entry map of string to string (alias to path)" on line %d`

// UnmarshalYAML is a custom unmarshaller for SecretSpec that allows us to
// unmarshal from different YAML node representations i.e. literal string or
// map.
func (t *SecretSpec) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return t.unmarshalFromLiteralString(node)
	case yaml.MappingNode:
		return t.unmarshalFromMap(node)
	}

	return fmt.Errorf(invalidSecretSpecErr, node.Line)
}

func (t *SecretSpec) unmarshalFromMap(node *yaml.Node) error {
	var mapValue map[string]string

	err := node.Decode(&mapValue)
	// Mapping node but not string to string
	if err != nil {
		// Adds a workaround for when content-type (node) exists under a variable path (string), i.e.
		// - conjur/var/encoded:
		//   content-type: base64
		err = t.tryUnmarshalMappedContentType(node)
		if err != nil {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}
		return nil
	}

	// Allow up to 2 entries if it contains the content-type as a mapping node
	// under the variable path
	if len(mapValue) != 1 && mapValue["content-type"] == "" || len(mapValue) > 2 {
		return fmt.Errorf(invalidSecretSpecErr, node.Line)
	}

	for k, v := range mapValue {
		if k == "content-type" {
			t.ContentType = v
		} else {
			t.Path = v
			t.Alias = k
		}
	}

	if t.ContentType == "" {
		t.ContentType = "text"
	}

	return nil
}

func (t *SecretSpec) tryUnmarshalMappedContentType(node *yaml.Node) error {
	var pathMap map[string]yaml.Node
	err := node.Decode(&pathMap)
	if err != nil || len(pathMap) != 1 {
		return fmt.Errorf(invalidSecretSpecErr, node.Line)
	}

	for path, content := range pathMap {
		t.Path = path
		t.Alias = path[strings.LastIndex(path, "/")+1:]
		if content.Kind != yaml.MappingNode {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}

		var contentTypeMap map[string]string
		err = content.Decode(&contentTypeMap)
		if err != nil || len(contentTypeMap) != 1 {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}

		if contentTypeMap["content-type"] != "" {
			t.ContentType = contentTypeMap["content-type"]
		} else {
			t.ContentType = "text"
		}
	}
	return nil
}

func (t *SecretSpec) unmarshalFromLiteralString(node *yaml.Node) error {
	var literalValue string
	err := node.Decode(&literalValue)

	// Scalar node but not a string
	if err != nil {
		return fmt.Errorf(invalidSecretSpecErr, node.Line)
	}

	t.Path = literalValue
	t.Alias = literalValue[strings.LastIndex(literalValue, "/")+1:]
	// Always assume text if no content-type mapping is present
	t.ContentType = "text"
	return nil
}

// NewSecretSpecs creates a slice of SecretSpec structs by unmarshalling
// a YAML representation of secret specifications.
func NewSecretSpecs(raw []byte) ([]SecretSpec, error) {
	var secretSpecs []SecretSpec
	err := yaml.Unmarshal(raw, &secretSpecs)
	if err != nil {
		return nil, fmt.Errorf("yaml: cannot unmarshall to list of secret specs: %v", err)
	}

	return secretSpecs, nil
}

func validateSecretPaths(secretSpecs []SecretSpec, groupName string) []error {
	var errors []error
	for _, secretSpec := range secretSpecs {
		if err := validateSecretPath(secretSpec.Path, groupName); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func validateSecretPath(path, groupName string) error {
	// The Conjur variable path must not be null
	if path == "" {
		return fmt.Errorf("Secret group %s: null Conjur variable path", groupName)
	}

	// The Conjur variable path must not end with slash character
	varName := path[strings.LastIndex(path, "/")+1:]
	if varName == "" {
		return fmt.Errorf("Secret group %s: the Conjur variable path '%s' has a trailing '/'",
			groupName, path)
	}

	// The Conjur variable name (the last word in the Conjur variable path)
	// must be no longer than 126 characters.
	if len(varName) > maxConjurVarNameLen {
		return fmt.Errorf("Secret group %s: the Conjur variable name '%s' is longer than %d characters",
			groupName, varName, maxConjurVarNameLen)
	}

	return nil
}
