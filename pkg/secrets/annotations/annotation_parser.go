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

		annotationsMap[key] = value
	}

	return annotationsMap, nil
}
