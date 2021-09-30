package pushtofile

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Temporary mocking of variable values and substitution expressions until
// support for retrieval from Conjur is added.
var mockExprValues = map[string]string{
	"$owner-name": "Mr. Spacely",
	"$owner-org":  "Spacely Space Sprockets, Inc.",
	"$db-server":  "prod-db-server",
	"$db-port":    "prod-db-port",
	"date":        time.Now().Format("2006-Jan-02"),
	"$db-cert | b64dec": `-----BEGIN CERTIFICATE-----
MIIDpTCCAo2gAwIBAgIRANdbd3Zw7nYF1dCvxYgatBcwDQYJKoZIhvcNAQELBQAw
GDEWMBQGA1UEAxMNY29uanVyLW9zcy1jYTAeFw0yMTA5MjQxODA2MzZaFw0yMjA5
MjQxODA2MzZaMBsxGTAXBgNVBAMTEGNvbmp1ci5teW9yZy5jb20wggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDLmeqODWjnWqimAS4cBCjCnP0TWvXuPtpn
FoweAZWBuAJQ6m9cWbCf11sqn44QdJ1RHcY8pa5CVduIqhSN7vLjTCBvF7mNI8n9
4rXBwwTQLHhj+cDSdFuyaIviVS7EIozmA+rfT5Km8nir1HnaeLmffD7ACKoAEAIS
k/nbeGKuCEbssjmPcZEQHXm7gLrszh5udleCAtS03f6L50zFWN8zW10/aE161qNv
JFqYViRoqZv4eitg7pxjmWtEj7KFu+6YN6/LFWbP0Qw/PblHACZcW5q+SWw494z2
yLOQk2KuSPIx19Z0z95LwOF9W47SHxUWs/qjYDFUCSAPyw2DBrABAgMBAAGjgeYw
geMwDgYDVR0PAQH/BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcD
AjAMBgNVHRMBAf8EAjAAMB8GA1UdIwQYMBaAFFo/I600mwyKdMGa65iVaec6gypF
MIGCBgNVHREEezB5ghBjb25qdXIubXlvcmcuY29tggpjb25qdXItb3NzghVjb25q
dXItb3NzLmNvbmp1ci1vc3OCGWNvbmp1ci1vc3MuY29uanVyLW9zcy5zdmOCJ2Nv
bmp1ci1vc3MuY29uanVyLW9zcy5zdmMuY2x1c3Rlci5sb2NhbDANBgkqhkiG9w0B
AQsFAAOCAQEASFVBwJBS0m9nA0msnEpAro7j4mHPFuDPsv7iS/jPRibk4Yq63xU5
SB5D144lo5HnjTqHQiYvkGhmpDivSuRUMpgcaWBseewWJiMeCmzlyO1ZTKqff7pf
KSWTX5LJz85LGsf4AqFbX1t9SW2B5GdQhLVdM5vGXx6f2pUXa3fyobzTOZtC2nW/
aSE+h1jKoaBCS5emZDFL7nXbJzBn+9P7TKXh/jZ89Sz70/1N6kKHI9QTlrkMibr9
9678MoOs2bI7stIDijwzYg/61Bv/rM+PvPSYP/Hhhr2OfXbrXzmj7FL4V58GZuJY
YrlGUaz+O8K1/bT/GSrFPSFZwGhJ0ZwmdQ==
-----END CERTIFICATE-----`,
}

//var sedCmds = `
//s/last modified.*/last modified {{date}}/
//s/name=.*/name={{$owner-name}}/
//s/organization=.*/organization={{$owner-org}}/
//s/server=.*/server={{$db-server}}/
//s/port=.*/name={{$db-port}}/
//s/cert=.*/cert="{{$db-cert | b64dec}}"/`

// ModifyConfigFiles iterates across all secrets groups and modifies
// config files for each group as specified via Pod Annotations.
func ModifyConfigFiles(secretGroups SecretGroups,
	secretValues map[string]string) error {
	for _, group := range secretGroups {
		err := modifyConfigFile(
			group.ConfigFileSrcPath,
			group.ConfigFileDestPath,
			group.ConfigFileMods,
			secretValues)
		if err != nil {
			return err
		}
	}
	return nil
}

func modifyConfigFile(srcFile, destFile, mods string, secretValues map[string]string) error {
	fmt.Printf("Modifying config file '%s' with modifications '%s'\n", srcFile, mods)

	err := copyFile(srcFile, destFile)
	if err != nil {
		return fmt.Errorf("error copying file '%s' to file '%s': %v", srcFile, destFile, err)
	}

	err = makeModifications(mods, destFile)
	if err != nil {
		return fmt.Errorf("error making modifications on %s: %v", destFile, err)
	}

	err = insertSecrets(secretValues, destFile)
	if err != nil {
		return fmt.Errorf("error retrieving and inserting Conjur secrets in %s: %v",
			destFile, err)
	}

	return nil
}

func copyFile(srcFile, destFile string) error {
	input, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(destFile, input, 0644)
	if err != nil {
		return err
	}

	return nil
}

func makeModifications(modsAnnotation, configFile string) error {
	// The Annotation value for the config file modifications will be of
	// the form of a YAML list. Unmarshall the Annotation value into a
	// slice of individual commands.
	var cmds []string
	err := yaml.Unmarshal([]byte(modsAnnotation), &cmds)
	if err != nil {
		return fmt.Errorf("error unmarshalling modification commands '%s': %v", err)
	}

	fmt.Printf("Running 'sed' commands '%v' on file %s...\n", cmds, configFile)
	for _, cmd := range cmds {
		if cmd == "" {
			continue
		}

		// Remove any leading or trailing quotes
		cmd = strings.TrimLeft(cmd, "\"'")
		cmd = strings.TrimRight(cmd, "\"'")

		fmt.Printf("Attempting to run: sed -i %s %s\n", cmd, configFile)
		cmd := exec.Command("sed", "-i", cmd, configFile)
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error running sed command: %v\n", err)
			return err
		}
	}
	return nil
}

func insertSecrets(secretValues map[string]string, configFile string) error {
	// For demo purposes, the 'secretValues' argument is ignored for now
	// (until support for retrieving secret values from Conjur is merged).
	// Instead, mock values are used to interpret modification expressions.

	// Regular expressions for search and replace
	findVars := regexp.MustCompile(`{{[^}]+}}`)
	findEscapedBraces := regexp.MustCompile(`\\[{}]`)

	// Read config file. This should contain substitution expressions
	// that are surrounded by double curly braces.
	input, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")
	for i, line := range lines {
		// Do search and replace for substitution expressions that are
		// enclosed in double curly braces.
		errorOccurred := false
		replaced := findVars.ReplaceAllFunc([]byte(line), func(s []byte) []byte {
			// Trim curly braces
			expr := strings.TrimLeft(string(s), "{")
			expr = strings.TrimRight(expr, "}")
			var value string
			var ok bool
			if value, ok = mockExprValues[expr]; !ok {
				fmt.Printf("Unknown variable or expression '$%s' in substitution Annotation '%s'\n",
					expr, line)
				errorOccurred = true
			}
			return []byte(value)
		})

		if errorOccurred == true {
			return fmt.Errorf("error(s) encountered in config file substitutions")
		}

		// Do a search and replace to restore any curly braces that had been
		// escaped with a preceding backslash.
		replaced = findEscapedBraces.ReplaceAllFunc(replaced, func(s []byte) []byte {
			// Trim leading backslash from escaped braces
			return []byte(strings.TrimLeft(string(s), "\\"))
		})

		lines[i] = string(replaced)
	}

	output := strings.Join(lines, "\n")
	return ioutil.WriteFile(configFile, []byte(output), 0644)
}
