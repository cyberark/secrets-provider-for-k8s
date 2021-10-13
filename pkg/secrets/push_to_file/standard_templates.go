package push_to_file

import "fmt"

type StandardTemplate struct {
	Template string
	FileFormat string
	Validator func(alias string) error
}

func (s StandardTemplate) ValidateAlias(alias string) error {
	if s.Validator == nil {
		return nil
	}

	return s.Validator(alias)
}

var standardTemplates = map[string]StandardTemplate{
	"yaml": { Template: yamlTemplate, Validator: func(alias string) error {
		return nil
	}},
	"json": { Template: jsonTemplate },
	"dotenv": { Template: dotenvTemplate },
	"bash": { Template: bashTemplate },
}

// FileTemplateForFormat returns the template for a file format, after ensuring the
// standard template exists and validating secret spec aliases against it
func FileTemplateForFormat(
	fileFormat string,
	secretSpecs []SecretSpec,
) (string, error) {
	stdTemplate, ok := standardTemplates[fileFormat]
	if !ok {
		return "", fmt.Errorf(`unrecognized standard file format, "%s"`, fileFormat)
	}

	for _, s := range secretSpecs {
		err := stdTemplate.ValidateAlias(s.Alias)
		if err != nil {
			return "", fmt.Errorf(
				"alias %q failed standard file format validation: %s",
				s.Alias,
				err,
			)
		}
	}

	return stdTemplate.Template, nil
}
