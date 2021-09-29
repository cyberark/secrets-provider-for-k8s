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

// UnmarshalYAML is a custom unmarshaller for SecretSpec that allows it to unmarshal
// from different yaml types by trying each one
func (t *SecretSpec) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	// LITERAL
	var literalValue string
	err := unmarshal(&literalValue)
	if err == nil {
		t.Path = literalValue
		t.Alias = literalValue[strings.LastIndex(literalValue, "/")+1:]
		return nil
	}

	// MAP
	var mapValue map[string]string
	err = unmarshal(&mapValue)
	if err == nil {
		if len(mapValue) != 1 {
			return fmt.Errorf("%s", "expected single key-value pair for secret specification")
		}

		for k, v := range mapValue {
			t.Path = v
			t.Alias = k
		}

		return nil
	}

	// ELSE
	return fmt.Errorf("%s", "unrecognized format for secret spec")
}

func NewSecretSpecs(raw []byte) ([]SecretSpec, error) {
	var secretSpecs []SecretSpec
	err := yaml.Unmarshal(raw, &secretSpecs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml list: %v", err)
	}

	return secretSpecs, nil
}
