package pushtofile

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
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
) (bool, error)

// openWriteCloserFunc is the func definition for openFileAsWriteCloser. It allows switching
// out openFileAsWriteCloser for a mock implementation
type openWriteCloserFunc func(
	path string,
	permissions os.FileMode,
) (io.WriteCloser, error)

// prevFileChecksums maps a secret group name to a sha256 checksum of the
// corresponding secret file content. This is used to detect changes in
// secret file content.
type checksum []byte

var prevFileChecksums = map[string]checksum{}

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
) (bool, error) {
	secretsMap := map[string]*Secret{}
	for _, s := range groupSecrets {
		secretsMap[s.Alias] = s
	}

	tpl, err := template.New(groupName).Funcs(template.FuncMap{
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
		"b64enc": b64encTemplateFunc,
		"b64dec": b64decTemplateFunc,
	}).Parse(groupTemplate)
	if err != nil {
		return false, err
	}

	if false {
		tpl.Execute(writer, templateData{
			SecretsArray: groupSecrets,
			SecretsMap:   secretsMap,
		})
		return false, nil
	}

	// Render the secret file content
	tplData := templateData{
		SecretsArray: groupSecrets,
		SecretsMap:   secretsMap,
	}
	fileContent, err := renderFile(tpl, tplData)
	if err != nil {
		return false, err
	}

	if writer == ioutil.Discard {
		_, err := writer.Write(fileContent.Bytes())
		return false, err
	}

	// Calculate a sha256 checksum on the content
	checksum, err := fileChecksum(fileContent)
	changed := contentHasChanged(groupName, checksum)

	// If file contents have changed, write the file and update checksum
	if changed {
		fmt.Printf("Writing secret file content: \n%s\n", fileContent.String())
		if _, err := writer.Write(fileContent.Bytes()); err != nil {
			return false, err
		}
		prevFileChecksums[groupName] = checksum
	}

	return changed, err
}

func renderFile(tpl *template.Template, tplData templateData) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	err := tpl.Execute(buf, tplData)
	return buf, err
}

// fileChecksum calculates a checksum on the file content
func fileChecksum(buf *bytes.Buffer) (checksum, error) {
	hash := sha256.New()
	bufCopy := bytes.NewBuffer(buf.Bytes())
	if _, err := io.Copy(hash, bufCopy); err != nil {
		return nil, err
	}
	checksum := hash.Sum(nil)
	return checksum, nil
}

func contentHasChanged(groupName string, fileChecksum checksum) bool {
	if prevChecksum, exists := prevFileChecksums[groupName]; exists {
		if bytes.Equal(fileChecksum, prevChecksum) {
			return false
		}
	}
	return true
}
