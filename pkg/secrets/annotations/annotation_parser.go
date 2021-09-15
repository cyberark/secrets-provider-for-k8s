package annotations

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// Define supported annotation keys for Secrets Provider config, as well as value restraints for each
var secretsProviderAnnotations = map[string][]string{
	"conjur.org/authn-identity":      {"string"},
	"conjur.org/container-mode":      {"string", "init", "application"},
	"conjur.org/secrets-destination": {"string", "file", "k8s_secret"},
	"conjur.org/k8s-secrets":         {"string"},
	"conjur.org/retry-count-limit":   {"int"},
	"conjur.org/retry-interval-sec":  {"int"},
	"conjur.org/debug-logging":       {"bool"},
}

// Define supported annotation key prefixes for Push to File config, as well as value restraints for each.
// In use, Push to File keys include a secret group ("conjur.org/conjur-secrets.{secret-group}").
// The values listed here will confirm hardcoded formatting, dynamic annotation content will
// be validated when used.
var pushToFileAnnotationPrefixes = map[string][]string{
	"conjur.org/conjur-secrets.":             {"string"},
	"conjur.org/conjur-secrets-policy-path.": {"string"},
	"conjur.org/secret-file-path.":           {"string"},
	"conjur.org/secret-file-format.":         {"string", "yaml", "json", "dotenv", "bash"},
	"conjur.org/secret-file-template.":       {"string"},
}

func AnnotationsFromFile(path string) (map[string]string, error) {
	annotationsFile, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, log.RecordedError(messages.CSPFK041E, err.Error())
	}
	defer annotationsFile.Close()
	return ParseAnnotations(annotationsFile)
}

// List and multi-line annotations are formatted as a single string in the annotations file,
// and this format persists into the Map returned by this function.
// For example, the following annotation:
//   conjur.org/conjur-secrets.cache: |
//     - url
//     - admin-password: password
//     - admin-username: username
// Is stored in the annotations file as:
//   conjur.org/conjur-secrets.cache="- url\n- admin-password: password\n- admin-username: username\n"
func ParseAnnotations(annotationsFile io.Reader) (map[string]string, error) {
	var lines []string
	scanner := bufio.NewScanner(annotationsFile)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	annotationsMap := make(map[string]string)
	for lineNumber, line := range lines {
		keyValuePair := strings.SplitN(line, "=", 2)
		if len(keyValuePair) == 1 {
			return nil, log.RecordedError(messages.CSPFK045E, lineNumber+1)
		}

		key := keyValuePair[0]
		value, err := strconv.Unquote(keyValuePair[1])
		if err != nil {
			return nil, log.RecordedError(messages.CSPFK045E, lineNumber+1)
		}

		// Validate key against known, properly formatted keys
		// Here, "match" and "foundMap" are used to make input validation map-agnostic
		if match, foundMap, ok := validateAnnotationKey(key); ok {
			acceptedValueInfo := foundMap[match]
			err := validateAnnotationValue(key, value, acceptedValueInfo)
			if err != nil {
				return nil, err
			} else {
				annotationsMap[key] = value
			}
		}
	}

	return annotationsMap, nil
}

func validateAnnotationKey(key string) (string, map[string][]string, bool) {
	// Validate that a given annotation key is formatted as "conjur.org/xyz"
	// Record Info level log if a key conforms to the formatting standard but
	// is not recognized as either a Secrets Provider config or Push to File config annotation
	//
	// If the annotation is for Push to File config, the ParseAnnotation function
	// needs to be aware of the annotation's valid prefix in order to perform input validation,
	// so this function returns:
	//   - either the key, or the key's valid prefix
	//   - the Map in which the key or prefix was found
	//   - the success status of the operation
	if !strings.HasPrefix(key, "conjur.org/") {
		return "", nil, false
	}

	if valueInMapKeys(key, secretsProviderAnnotations) {
		return key, secretsProviderAnnotations, true
	} else if prefix, ok := valuePrefixInMapKeys(key, pushToFileAnnotationPrefixes); ok {
		return prefix, pushToFileAnnotationPrefixes, true
	} else {
		log.Info(messages.CSPFK011I, key)
		return "", nil, false
	}
}

func valueInMapKeys(value string, searchMap map[string][]string) bool {
	if _, ok := searchMap[value]; ok {
		return true
	} else {
		return false
	}
}

func valuePrefixInMapKeys(value string, searchMap map[string][]string) (string, bool) {
	for key := range searchMap {
		if strings.HasPrefix(value, key) {
			return key, true
		}
	}
	return "", false
}

func valueInArray(value string, array []string) bool {
	for _, item := range array {
		if value == item {
			return true
		}
	}
	return false
}

func validateAnnotationValue(key string, value string, acceptedValueInfo []string) error {
	// given a key/value pair, where the key is confirmed to be a Secrets Provider config annotation
	// validated that the value is of valid type, or confirm that the value is
	// in the range of enumerated and acceptable values
	switch targetType := acceptedValueInfo[0]; targetType {
	case "int":
		_, err := strconv.Atoi(value)
		if err != nil {
			return log.RecordedError(messages.CSPFK042E, key, value, "Integer")
		}
	case "bool":
		if value != "true" && value != "false" {
			return log.RecordedError(messages.CSPFK042E, key, value, "Boolean")
		}
	case "string":
		acceptedValues := acceptedValueInfo[1:]
		if len(acceptedValues) > 0 && !valueInArray(value, acceptedValues) {
			return log.RecordedError(messages.CSPFK043E, key, value, acceptedValues)
		}
	}
	return nil
}
