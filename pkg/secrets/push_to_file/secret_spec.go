package push_to_file

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type SecretSpec struct {
	Alias string
	Path  string
}

func (t SecretSpec) MarshalYAML() (interface{}, error) {
	return map[string]string{t.Alias: t.Path}, nil
}

const invalidSecretSpecErr = `expected a "string (path)" or "single entry map of string to string (alias to path)" on line %d`

// UnmarshalYAML is a custom unmarshaller for SecretSpec that allows it to unmarshal
// from different yaml types by trying each one
func (t *SecretSpec) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var literalValue string
		err := node.Decode(&literalValue)

		// Scalar node but not a string
		if err != nil {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}

		t.Path = literalValue
		t.Alias = literalValue[strings.LastIndex(literalValue, "/")+1:]
		return nil
	case yaml.MappingNode:
		var mapValue map[string]string

		err := node.Decode(&mapValue)
		// Mapping node but not string to string
		if err != nil {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}

		// Mapping node but has multiple entries
		if len(mapValue) != 1 {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}

		for k, v := range mapValue {
			t.Path = v
			t.Alias = k
		}

		return nil
	}

	return fmt.Errorf(invalidSecretSpecErr, node.Line)
}

func NewSecretSpecs(raw []byte) ([]SecretSpec, error) {
	var secretSpecs []SecretSpec
	err := yaml.Unmarshal(raw, &secretSpecs)
	if err != nil {
		return nil, fmt.Errorf("yaml: cannot unmarshall to list of secret specs: %v", err)
	}

	return secretSpecs, nil
}
