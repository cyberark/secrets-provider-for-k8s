package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	_, b, _, _ = runtime.Caller(0)
	workingdir, _ = os.Getwd()
	basepath   = filepath.Dir(b)
)

// Taken from Kubernetes source used to generate annotation file for downward API
// https://github.com/kubernetes/kubernetes/blob/master/pkg/fieldpath/fieldpath.go#L28-L41
func FormatMap(m map[string]string) (fmtStr string) {
	for k, v := range m {
		fmtStr += fmt.Sprintf("%v=%q\n", k, v)
	}
	fmtStr = strings.TrimSuffix(fmtStr, "\n")

	return
}

func main() {
	annotations := FormatMap(map[string]string{
		"conjur.org/conjur-secrets.cache": `- dev/redis/api-url
- admin-username: dev/redis/username
- admin-password: dev/redis/password
`,
		"conjur.org/secret-file-path.cache": "./testdata/redis.json",
		"conjur.org/secret-file-format.cache": "json",
		"conjur.org/conjur-secrets.db": `- url
- password
- username
`,
		"conjur.org/conjur-secrets-policy-path.db": `dev/database`,
		"conjur.org/secret-file-path.db": "./testdata/db.js",
		"conjur.org/secret-file-template.db": `
export const url={{ printf "%q" (secret "password") }}
export const username={{ printf "%q" (secret "password") }}
export const password={{ printf "%q" (secret "password") }}
`,
	})

	annotationsFilePath, _ := filepath.Rel(workingdir, filepath.Join(basepath, "./annotations.txt"))
	fmt.Println("Generating " + annotationsFilePath)
	_ = ioutil.WriteFile(annotationsFilePath, []byte(annotations), 0644)
}