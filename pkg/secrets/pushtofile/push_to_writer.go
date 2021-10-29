package pushtofile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"
)

// templateData describes the form in which data is presented to push-to-file templates
type templateData struct {
	SecretsArray []*Secret
	SecretsMap   map[string]*Secret
}

// pushToWriterFunc is the func definition for pushToWriter. It allows switching out pushToWriter
// for a mock implementation
type pushToWriterFunc func(
	writer io.Writer,
	groupName string,
	groupTemplate string,
	groupSecrets []*Secret,
) error

// openWriteCloserFunc is the func definition for openFileAsWriteCloser. It allows switching
// out openFileAsWriteCloser for a mock implementation
type openWriteCloserFunc func(
	path string,
	permissions os.FileMode,
) (io.WriteCloser, error)

// openFileAsWriteCloser opens a file to write-to with some permissions.
func openFileAsWriteCloser(path string, permissions os.FileMode) (io.WriteCloser, error) {
	dir := filepath.Dir(path)

	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("unable to mkdir when opening file to write at %q: %s", path, err)
	}

	wc, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permissions)
	if err != nil {
		return nil, fmt.Errorf("unable to open file to write at %q: %s", path, err)
	}

	return wc, nil
}

// pushToWriter takes a (group's) path, template and secrets, and processes the template
// to generate text content that is pushed to a writer. push-to-file wraps around this.
func pushToWriter(
	writer io.Writer,
	groupName string,
	groupTemplate string,
	groupSecrets []*Secret,
) error {
	secretsMap := map[string]*Secret{}
	for _, s := range groupSecrets {
		secretsMap[s.Alias] = s
	}

	t, err := template.New(groupName).Funcs(template.FuncMap{
		// secret is a custom utility function for streamlined access to secret values.
		// It panics for secrets aliases not specified on the group.
		"secret": func(alias string) string {
			v, ok := secretsMap[alias]
			if ok {
				return v.Value
			}

			// Panic in a template function is captured as an error
			// when the template is executed.
			panic(fmt.Sprintf("secret alias %q not present in specified secrets for group", alias))
		},
	}).Parse(groupTemplate)
	if err != nil {
		return err
	}

	return t.Execute(writer, templateData{
		SecretsArray: groupSecrets,
		SecretsMap:   secretsMap,
	})
}
