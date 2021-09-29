package push_to_file

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
