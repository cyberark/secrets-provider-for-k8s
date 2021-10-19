package pushtofile

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	"gopkg.in/yaml.v3"
)

const (
	maxConjurVarNameLen = 126
	maxYAMLKeyLen       = 1024
	maxJSONKeyLen       = 2097152
)

// SecretSpec specifies a secret to be retrieved from Conjur by defining
// its alias (i.e. the name of the secret from an application's perspective)
// and its variable path in Conjur.
type SecretSpec struct {
	Alias string
	Path  string
}

// MarshalYAML is a custom marshaller for SecretSpec.
func (t SecretSpec) MarshalYAML() (interface{}, error) {
	return map[string]string{t.Alias: t.Path}, nil
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
	t.Alias = literalValue[strings.LastIndex(literalValue, "/")+1:]
	return nil
}

func (t *SecretSpec) unmarshalFromMap(node *yaml.Node) error {
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

func validateSecretSpecs(secretSpecs []SecretSpec, fileFormat string) error {
	invalidSpecsFound := false
	for _, secretSpec := range secretSpecs {
		if err := validateSecretSpec(secretSpec, fileFormat); err != nil {
			invalidSpecsFound = true
			log.RecordedError(messages.CSPFK053E, secretSpec.Alias, err.Error())
		}
	}
	if invalidSpecsFound {
		return errors.New("invalid secret specifications found in Annotations")
	}
	return nil
}

func validateSecretSpec(secretSpec SecretSpec, fileFormat string) error {
	if err := validateSecretPath(secretSpec.Path); err != nil {
		return err
	}
	return validateSecretAlias(secretSpec.Alias, fileFormat)
}

func validateSecretPath(path string) error {
	// The Conjur variable path must not be null
	if path == "" {
		return errors.New("null Conjur variable path")
	}

	// The Conjur variable path must not end with slash character
	varName := path[strings.LastIndex(path, "/")+1:]
	if varName == "" {
		return fmt.Errorf("the Conjur variable path '%s' has a trailing '/'", path)
	}

	// The Conjur variable name (the last word in the Conjur variable path)
	// must be no longer than 126 characters.
	if len(varName) > maxConjurVarNameLen {
		return fmt.Errorf("the Conjur variable name '%s' is longer than %d characters",
			varName, maxConjurVarNameLen)
	}

	return nil
}

func validateSecretAlias(alias, fileFormat string) error {
	// Check for null alias
	if alias == "" {
		return errors.New("null secret alias")
	}

	// Validate the secret alias based on the secret file format
	type aliasValidator func(string) error
	validators := map[string](aliasValidator){
		"yaml":   checkValidYAMLKey,
		"json":   checkValidJSONKey,
		"bash":   checkValidBashVarName,
		"dotenv": checkValidBashVarName, // Same limitations as Bash
	}
	if validator, ok := validators[fileFormat]; ok {
		return validator(alias)
	}

	// Assuming either 'plaintext' file format or custom template is being
	// used for this secret group. For these cases, any string is acceptable.
	return nil
}

func checkValidYAMLKey(key string) error {
	if len(key) > maxYAMLKeyLen {
		return fmt.Errorf("the key '%s' is too long for YAML", key)
	}
	for _, c := range key {
		if !isValidYAMLChar(c) {
			return fmt.Errorf("invalid YAML character: '%c'", c)
		}
	}
	return nil
}

func isValidYAMLChar(c rune) bool {
	// Checks whether a character is in the YAML valid character set as
	// defined here: https://yaml.org/spec/1.2.2/#51-character-set
	switch {
	case c == '\u0009':
		return true // tab
	case c == '\u000A':
		return true // LF
	case c == '\u000D':
		return true // CR
	case c >= '\u0020' && c <= '\u007E':
		return true // Printable ASCII
	case c == '\u0085':
		return true // Next Line (NEL)
	case c >= '\u00A0' && c <= '\uD7FF':
		return true // Basic Multilingual Plane (BMP)
	case c >= '\uE000' && c <= '\uFFFD':
		return true // Additional Unicode Areas
	case c >= '\U00010000' && c <= '\U0010FFFF':
		return true // 32 bit
	default:
		return false
	}
}

func checkValidJSONKey(key string) error {
	if len(key) > maxJSONKeyLen {
		return fmt.Errorf("the key '%s' is too long for JSON", key)
	}
	for _, c := range key {
		if !isValidJSONChar(c) {
			return fmt.Errorf("invalid JSON character: '%c'", c)
		}
	}
	return nil
}

func isValidJSONChar(c rune) bool {
	// Checks whether a character is in the JSON valid character set as
	// defined here: https://www.json.org/json-en.html
	// This document specifies that any characters are valid except:
	//   - Control characters (0x00-0x1F and 0x7f [DEL])
	//   - Double quote (")
	//   - Backslash (\)
	switch {
	case c >= '\u0000' && c <= '\u001F':
		return false // Control characters other than DEL
	case c == '\u007F':
		return false // DEL
	case c == rune('"'):
		return false // Double quote
	case c == rune('\\'):
		return false // Backslash
	default:
		return true
	}
}

func checkValidBashVarName(name string) error {
	r := regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")
	if !r.MatchString(name) {
		explanation := "Must be alphanumerics and underscores, with first char being a non-digit"
		return fmt.Errorf("invalid alias '%s': %s", name, explanation)
	}
	return nil
}
