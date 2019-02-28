package executers

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetPackageZipLocation(t *testing.T) {
	baseDir, err := getBaseDir()
	if err != nil {
		t.Error(err)
	}
	cachePath := filepath.Join(baseDir, "zip", "download")
	tests := []struct {
		dependencyName string
		version        string
		expectedPath   string
	}{
		{"rsc.io/quote", "v1.5.2", filepath.Join(filepath.Dir(cachePath), "rsc.io", "quote", "@v", "v1.5.2.zip")},
		{"rsc/quote", "v1.5.3", filepath.Join(cachePath, "rsc", "quote", "@v", "v1.5.3.zip")},
		{"rsc.io/quote", "v1.5.4", ""},
	}

	for _, test := range tests {
		t.Run(test.dependencyName+":"+test.version, func(t *testing.T) {
			actual, err := getPackageZipLocation(cachePath, test.dependencyName, test.version)
			if err != nil {
				t.Error(err.Error())
			}

			if test.expectedPath != actual {
				t.Errorf("Test name: %s:%s: Expected: %s, Got: %s", test.dependencyName, test.version, test.expectedPath, actual)
			}
		})
	}
}

func TestGetDependencyToLowerCase(t *testing.T) {
	tests := []struct {
		dependencyName string
		expectedPath   string
	}{
		{"github.com/Sirupsen/logrus", "github.com/!sirupsen/logrus"},
		{"Rsc/quOte", "!rsc/qu!ote"},
		{"golang.org/x/crypto", "golang.org/x/crypto"},
		{"golang.org/X/crypto", "golang.org/!x/crypto"},
		{"rsc.io/quote", "rsc.io/quote"},
	}

	for _, test := range tests {
		t.Run(test.dependencyName, func(t *testing.T) {
			actual := getDependencyToLowerCase(test.dependencyName)
			if test.expectedPath != actual {
				t.Errorf("Test name: %s: Expected: %s, Got: %s", test.dependencyName, test.expectedPath, actual)
			}
		})
	}
}

func TestCreateDependency(t *testing.T) {
	err := fileutils.CreateTempDirPath()
	if err != nil {
		t.Error(err)
	}
	defer fileutils.RemoveTempDir()
	baseDir, err := getBaseDir()
	if err != nil {
		t.Error(err)
	}
	cachePath := filepath.Join(baseDir, "zip")
	modContent := "module github.com/test"
	dep := Package{
		id:         "github.com/test:v1.2.3",
		modContent: []byte(modContent),
		zipPath:    filepath.Join(cachePath, "v1.2.3.zip"),
	}
	tempDir, err := createDependencyInTemp(dep.GetZipPath())
	if err != nil {
		t.Error(err)
	}
	path := filepath.Join(tempDir, "github.com", "test@v1.2.3", "test.go")

	exists, err := fileutils.IsFileExists(path, false)
	if err != nil {
		t.Error(err)
	}

	if !exists {
		t.Error(fmt.Sprintf("Missing %s", path))
	}
	err = os.RemoveAll(filepath.Join(tempDir, "github.com"))
	if err != nil {
		t.Error(err)
	}
}

func TestGetModPath(t *testing.T) {
	err := fileutils.CreateTempDirPath()
	if err != nil {
		t.Error(err)
	}
	defer fileutils.RemoveTempDir()
	baseDir, err := getBaseDir()
	if err != nil {
		t.Error(err)
	}
	cachePath := filepath.Join(baseDir, "zip")
	modContent := "module github.com/test"
	dep := Package{
		id:         "github.com/test:v1.2.3",
		modContent: []byte(modContent),
		zipPath:    filepath.Join(cachePath, "v1.2.3.zip"),
	}
	pwd := PackageWithDeps{Dependency: &dep}
	tempDir, err := createDependencyInTemp(dep.GetZipPath())
	if err != nil {
		t.Error(err)
	}
	modPath := pwd.getModPathInTemp(tempDir)
	path := filepath.Join(tempDir, "github.com", "test@v1.2.3", "go.mod")
	if path != modPath {
		t.Error(fmt.Sprintf("Expected %s, got %s", path, modPath))
	}

	err = os.RemoveAll(filepath.Join(tempDir, "github.com"))
	if err != nil {
		t.Error(err)
	}
}

func TestMergeReplaceDependenciesWithGraphDependencies(t *testing.T) {
	tests := []struct {
		name              string
		replaceDeps       []string
		graphDependencies map[string]bool
		expectedMap       map[string]bool
	}{
		{"missingInGraphMap", []string{"replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.1.0", "replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-cli-go", "replace github.com/jfrog/jfrog-client-go => /path/to/mod/file"}, map[string]bool{},
			map[string]bool{"github.com/jfrog/jfrog-client-go@v0.1.0": true}},
		{"existsInGraphMap", []string{"replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.1.0", "replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-cli-go", "replace github.com/jfrog/jfrog-client-go => /path/to/mod/file"}, map[string]bool{"github.com/jfrog/jfrog-client-go@v0.1.0": true},
			map[string]bool{"github.com/jfrog/jfrog-client-go@v0.1.0": true}},
		{"addToGraphMap", []string{"replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.1.0", "replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-cli-go", "replace github.com/jfrog/jfrog-client-go => /path/to/mod/file"}, map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true},
			map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true, "github.com/jfrog/jfrog-client-go@v0.1.0": true}},
		{"addToGraphMapFromReplaceBlock", []string{"replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.1.0", "replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-cli-go", "replace github.com/jfrog/jfrog-client-go => /path/to/mod/file", "github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v2.1.2"}, map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true},
			map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true, "github.com/jfrog/jfrog-client-go@v0.1.0": true, "github.com/jfrog/jfrog-client-go@v2.1.2": true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mergeReplaceDependenciesWithGraphDependencies(test.replaceDeps, test.graphDependencies)
			if !reflect.DeepEqual(test.expectedMap, test.graphDependencies) {
				t.Errorf("Test name: %s: Expected: %v, Got: %v", test.name, test.expectedMap, test.graphDependencies)
			}
		})
	}
}

func TestParseModForReplaceDependencies(t *testing.T) {
	tests := []struct {
		name                        string
		modContent                  string
		expectedReplaceDependencies []string
	}{
		{"replaceBlockFirst", `module jfrog.com/jfrog-router

replace (
        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1
)

require (
        code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
        contrib.go.opencensus.io/exporter/ocagent v0.4.6 // indirect
)
`, []string{"        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceBlockLast", `module jfrog.com/jfrog-router

require (
        code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
        contrib.go.opencensus.io/exporter/ocagent v0.4.6 // indirect
)

replace (
        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1
)


`, []string{"        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceLineFirst", `module jfrog.com/jfrog-router

replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1

require (
        code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
        contrib.go.opencensus.io/exporter/ocagent v0.4.6 // indirect
)
`, []string{"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceLineLast", `module jfrog.com/jfrog-router

require (
        code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
        contrib.go.opencensus.io/exporter/ocagent v0.4.6 // indirect
)

replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1

`, []string{"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceBothLineFirst", `module jfrog.com/jfrog-router

require (
        code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
        contrib.go.opencensus.io/exporter/ocagent v0.4.6 // indirect
)

replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1

replace (
        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1
)

`, []string{"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1", "        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceBothBlockFirst", `module jfrog.com/jfrog-router

require (
        code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
        contrib.go.opencensus.io/exporter/ocagent v0.4.6 // indirect
)

replace (
        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1
)

replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible
replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1


`, []string{"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1", "        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible", "        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			replaceLinerDependencies, err := parseModForReplaceDependencies(test.modContent)
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(test.expectedReplaceDependencies, replaceLinerDependencies) {
				t.Errorf("Test name: %s: Expected: %v, Got: %v", test.name, test.expectedReplaceDependencies, replaceLinerDependencies)
			}
		})
	}
}

func getBaseDir() (baseDir string, err error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	pwd = filepath.Dir(pwd)
	baseDir = filepath.Join(pwd, "testdata")
	return
}
