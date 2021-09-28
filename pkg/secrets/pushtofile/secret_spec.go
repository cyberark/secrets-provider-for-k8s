package pushtofile

import (
	"fmt"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"gopkg.in/yaml.v3"
)

type SecretSpec struct {
	Id    string
	Alias string
}

func (t SecretSpec) MarshalYAML() (interface{}, error) {
	return map[string]string{t.Alias: t.Id}, nil
}

// UnmarshalYAML is a custom unmarshaller for SecretSpec that allows it to unmarshal
// from different yaml types by trying each one
func (t *SecretSpec) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	// LITERAL
	var literalValue string
	err := unmarshal(&literalValue)
	if err == nil {
		t.Id = literalValue
		t.Alias = literalValue[strings.LastIndex(literalValue, "/")+1:]
		return nil
	}

	// MAP
	var mapValue map[string]string
	err = unmarshal(&mapValue)
	if err == nil {
		if len(mapValue) != 1 {
			return log.RecordedError(messages.CSPFK051E, "expected single key-value pair for secret specification")
		}

		for k, v := range mapValue {
			t.Id = v
			t.Alias = k
		}

		return nil
	}

	// ELSE
	return log.RecordedError(messages.CSPFK051E, "unrecognized format for secret spec")
}

func NewSecretSpecs(raw []byte) ([]SecretSpec, error) {
	var secretSpecs []SecretSpec
	err := yaml.Unmarshal(raw, &secretSpecs)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK051E, fmt.Sprintf("failed to parse yaml list: %v", err))
	}

	return secretSpecs, nil
}
