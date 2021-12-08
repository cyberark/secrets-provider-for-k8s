package pushtofile

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig"
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

	wc, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, permissions)
	if err != nil {
		return nil, fmt.Errorf("unable to open file to write at %q: %s", path, err)
	}

	//Chmod file to set permissions regardless of 'umask'
	if err := os.Chmod(path, permissions); err != nil {
		return nil, fmt.Errorf("unable to chmod file %q: %s", path, err)
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

	t, err := template.New(groupName).Funcs(sprig.TxtFuncMap()).Funcs(template.FuncMap{
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
		// b64enc is a custom utility function for performing a base64 encode
		// on a secret value.
		"b64enc": func(value string) string {
			return base64.StdEncoding.EncodeToString([]byte(value))
		},
		// b64dec is a custom utility function for performing a base64 decode
		// on a secret value.
		"b64dec": func(encValue string) string {
			decValue, err := base64.StdEncoding.DecodeString(encValue)
			if err == nil {
				return string(decValue)
			}

			// Panic in a template function is captured as an error
			// when the template is executed.
			panic(fmt.Sprintf("value could not be base64 decoded"))
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
