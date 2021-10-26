package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
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
	contents, err := ioutil.ReadFile("./annotations.yml")
	if err != nil {
		panic(err)
	}

	var annotationsMap map[string]string
	err = yaml.Unmarshal(contents, &annotationsMap)
	if err != nil {
		panic(err)
	}

	annotations := FormatMap(annotationsMap)

	annotationsFilePath, _ := filepath.Rel(workingdir, filepath.Join(basepath, "./annotations.txt"))
	fmt.Println("Generating " + annotationsFilePath)
	_ = ioutil.WriteFile(annotationsFilePath, []byte(annotations), 0644)
}
