package filetemplates

import (
	"fmt"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/utils"
	"gopkg.in/yaml.v3"
)

const (
	MaxConjurVarNameLen = 126
)

// SecretSpec specifies a secret to be retrieved from Conjur by defining
// its alias (i.e. the name of the secret from an application's perspective)
// and its variable path in Conjur.
type SecretSpec struct {
	Alias            string
	Path             string
	ContentType      string
	TrimPathSegments int
}

// SecretGroup incorporates common information about a secret group
// that has been parsed from that secret group's Annotations.
type SecretGroup struct {
	Name         string
	FileTemplate string
	SecretSpecs  []SecretSpec
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

func (t *SecretSpec) unmarshalFromLiteralString(node *yaml.Node) error {
	var literalValue string
	err := node.Decode(&literalValue)

	// Scalar node but not a string
	if err != nil {
		return fmt.Errorf(invalidSecretSpecErr, node.Line)
	}

	t.Path = literalValue
	// When no alias is provided, use the last part of the variable path as the alias
	t.Alias = literalValue[strings.LastIndex(literalValue, "/")+1:]
	t.ContentType = "text"
	return nil
}

func (t *SecretSpec) unmarshalFromMap(node *yaml.Node) error {
	var mapValue map[string]interface{}

	err := node.Decode(&mapValue)
	// Mapping node but not string to string
	if err != nil {
		return fmt.Errorf(invalidSecretSpecErr, node.Line)
	}

	t.ContentType = "text"
	count := 0
	for k, v := range mapValue {
		if k == "content-type" {
			contentType, ok := v.(string)
			if !ok {
				return fmt.Errorf(invalidSecretSpecErr, node.Line)
			}
			t.ContentType = contentType
			continue
		}
		if k == "trim-path-segments" {
			trimPathSegments, err := utils.ParseTrimPathSegments(v)
			if err != nil {
				return fmt.Errorf("secret spec has invalid trim-path-segments: %v", err)
			}
			t.TrimPathSegments = trimPathSegments
			continue
		}

		path, ok := v.(string)
		if !ok {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}
		count = count + 1
		if count > 1 {
			return fmt.Errorf(invalidSecretSpecErr, node.Line)
		}
		t.Path = path
		t.Alias = k
	}
	return nil
}

// NewSecretSpecs creates a slice of SecretSpec structs by unmarshalling
// a YAML representation of secret specifications.
func NewSecretSpecs(raw []byte) ([]SecretSpec, error) {
	var secretSpecs []SecretSpec

	// Support just the string "*" instead of a list
	if string(raw) == "*" {
		return []SecretSpec{
			{
				Path:        "*",
				Alias:       "*",
				ContentType: "text",
			},
		}, nil
	}

	err := yaml.Unmarshal(raw, &secretSpecs)
	if err != nil {
		return nil, fmt.Errorf("yaml: cannot unmarshall to list of secret specs: %v", err)
	}

	return secretSpecs, nil
}

func ValidateSecretPathsAndContents(secretSpecs []SecretSpec, groupName string) []error {

	errors := ValidateSecretPaths(secretSpecs, groupName)

	errs := ValidateSecretContents(secretSpecs, groupName)
	for _, err := range errs {
		// Log the errors as warnings but allow it to proceed
		log.Warn(messages.CSPFK065E, err.Error())
	}

	errors = append(errors, ValidateSecretTrimPathSegments(secretSpecs, groupName)...)
	return errors
}

func ValidateSecretTrimPathSegments(secretSpecs []SecretSpec, groupName string) []error {
	var errors []error
	for _, secretSpec := range secretSpecs {
		if err := validateSecretTrimPathSegments(secretSpec.TrimPathSegments, groupName); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func validateSecretTrimPathSegments(trimPathSegments int, groupName string) error {
	if trimPathSegments < 0 {
		return fmt.Errorf("Secret group %s: trim-path-segments expected a non-negative integer, got %d", groupName, trimPathSegments)
	}
	return nil
}

func ValidateSecretPaths(secretSpecs []SecretSpec, groupName string) []error {
	var errors []error
	for _, secretSpec := range secretSpecs {
		if err := validateSecretPath(secretSpec.Path, groupName); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func ValidateSecretContents(secretSpecs []SecretSpec, groupName string) []error {
	var errors []error
	for _, secretSpec := range secretSpecs {
		if err := validateSecretContent(secretSpec.ContentType, groupName); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func validateSecretContent(content, groupName string) error {
	if content == "text" || content == "base64" {
		return nil
	} else {
		return fmt.Errorf(messages.CSPFK065E, groupName, content)
	}
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
	if len(varName) > MaxConjurVarNameLen {
		return fmt.Errorf("Secret group %s: the Conjur variable name '%s' is longer than %d characters",
			groupName, varName, MaxConjurVarNameLen)
	}

	return nil
}
