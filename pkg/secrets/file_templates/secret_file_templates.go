package filetemplates

import (
	"bytes"
	"fmt"
	"text/template"
)

const SecretGroupPrefix = "conjur.org/conjur-secrets."
const SecretGroupFileTemplatePrefix = "conjur.org/secret-file-template."

// Secret describes how Conjur secrets are represented in the file-template-rendering context.
type Secret struct {
	Alias string
	Value string
}

// templateData describes the form in which data is presented to file templates
type TemplateData struct {
	SecretsArray []*Secret
	SecretsMap   map[string]*Secret
}

func RenderFile(tpl *template.Template, tplData TemplateData) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	err := tpl.Execute(buf, tplData)
	return buf, err
}

func GetTemplate(name string, secretsMap map[string]*Secret) *template.Template {

	return template.New(name).Funcs(template.FuncMap{
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
	})
}
