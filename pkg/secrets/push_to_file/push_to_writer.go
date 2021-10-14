package push_to_file

import (
	"io"
	"os"
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

// openWriteCloserFunc is the func definition for openFileToWriterCloser. It allows switching
// out openFileToWriterCloser for a mock implementation
type openWriteCloserFunc func(
	path string,
	permissions os.FileMode,
) (io.WriteCloser, error)

// openFileToWriterCloser opens a file to write-to with some permissions.
func openFileToWriterCloser(path string, permissions os.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permissions)
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
		// It panics for errors not specified on the group.
		"secret": func(label string) string {
			v, ok := secretsMap[label]
			if ok {
				return v.Value
			}

			// Panic in a template function is captured as an error
			// when the template is executed.
			panic("secret alias not present in specified secrets for group")
		},
		// toYaml marshals a given value to YAML
		//"toYaml": func(value interface{}) string {
		//	d, err := yaml.Marshal(&value)
		//	if err != nil {
		//		panic(err)
		//	}
		//
		//	return string(d)
		//},
	}).Parse(groupTemplate)
	if err != nil {
		return err
	}

	return t.Execute(writer, templateData{
		SecretsArray: groupSecrets,
		SecretsMap:   secretsMap,
	})
}

