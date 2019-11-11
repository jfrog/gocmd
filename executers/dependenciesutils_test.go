package executers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func TestGetPackageZipLocation(t *testing.T) {
	log.SetLogger(log.NewLogger(log.DEBUG, nil))
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

func TestEncodeDecodePath(t *testing.T) {
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
			encoded := goModEncode(test.dependencyName)
			if test.expectedPath != encoded {
				t.Errorf("Test name: %s: Expected: %s, Got: %s", test.dependencyName, test.expectedPath, encoded)
			}
			decoded := goModDecode(test.expectedPath)
			if test.dependencyName != decoded {
				t.Errorf("Test name: %s: Expected: %s, Got: %s", test.dependencyName, test.dependencyName, decoded)
			}
		})
	}
}

func TestCreateDependency(t *testing.T) {
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		t.Error(err)
	}
	defer fileutils.RemoveTempDir(tempDirPath)
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
	err = createDependencyInTemp(dep.GetZipPath(), tempDirPath)
	if err != nil {
		t.Error(err)
	}
	path := filepath.Join(tempDirPath, "github.com", "test@v1.2.3", "test.go")

	exists, err := fileutils.IsFileExists(path, false)
	if err != nil {
		t.Error(err)
	}

	if !exists {
		t.Error(fmt.Sprintf("Missing %s", path))
	}
	err = os.RemoveAll(filepath.Join(tempDirPath, "github.com"))
	if err != nil {
		t.Error(err)
	}
}

func TestGetModPath(t *testing.T) {
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		t.Error(err)
	}
	defer fileutils.RemoveTempDir(tempDirPath)
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
	pwd := PackageWithDeps{Dependency: &dep, depsTempDir: tempDirPath}
	err = createDependencyInTemp(dep.GetZipPath(), tempDirPath)
	if err != nil {
		t.Error(err)
	}
	modPath := pwd.getModPathInTemp(tempDirPath)
	path := filepath.Join(tempDirPath, "github.com", "test@v1.2.3", "go.mod")
	if path != modPath {
		t.Error(fmt.Sprintf("Expected %s, got %s", path, modPath))
	}

	err = os.RemoveAll(filepath.Join(tempDirPath, "github.com"))
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
		{"missingInGraphMap",
			[]string{
				"replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.1.0",
			},
			map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true},
			map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true}},
		{"existsInGraphMap",
			[]string{"replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.1.0"},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.1.0": true},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.1.0": true}},
		{"replaceInGraphMapOneMatch",
			[]string{"github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.2.0"},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.1.0": true},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.2.0": true}},
		{"replaceInGraphMapOneMatchExactVersion",
			[]string{"replace github.com/jfrog/jfrog-client-go v0.1.1 => github.com/jfrog/jfrog-client-go v0.2.0"},
			map[string]bool{
				"github.com/jfrog/jfrog-client-go@v0.1.0": true,
				"github.com/jfrog/jfrog-client-go@v0.1.1": true},
			map[string]bool{
				"github.com/jfrog/jfrog-client-go@v0.1.0": true,
				"github.com/jfrog/jfrog-client-go@v0.2.0": true}},
		{"replaceInGraphMapMultipleMatch",
			[]string{"replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.2.0"},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.1.0": true,
				"github.com/jfrog/jfrog-client-go@v0.1.1": true},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.2.0": true}},
		{"missingInGraphMapLocalReplace",
			[]string{
				"replace github.com/jfrog/jfrog-client-go => ../jfrog-client-go",
			},
			map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true},
			map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true}},
		{"existsInGraphMapOneMatchLocalReplace",
			[]string{"replace github.com/jfrog/jfrog-client-go => ../jfrog-client-go"},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.1.0": true},
			map[string]bool{"github.com/jfrog/jfrog-cli-go@v1.21.0": true}},
		{"replaceInGraphMapOneMatchExactVersionLocalReplace",
			[]string{"replace github.com/jfrog/jfrog-client-go v0.1.1 => ../jfrog-client-go"},
			map[string]bool{
				"github.com/jfrog/jfrog-client-go@v0.1.0": true,
				"github.com/jfrog/jfrog-client-go@v0.1.1": true},
			map[string]bool{"github.com/jfrog/jfrog-client-go@v0.1.0": true}},
		{"replaceInGraphMapMultipleMatchLocalReplace",
			[]string{"replace github.com/jfrog/jfrog-client-go => ../jfrog-client-go"},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0":   true,
				"github.com/jfrog/jfrog-client-go@v0.1.0": true,
				"github.com/jfrog/jfrog-client-go@v0.1.1": true},
			map[string]bool{
				"github.com/jfrog/jfrog-cli-go@v1.21.0": true}},
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
	testdata, err := getBaseDir()
	if err != nil {
		t.Error(err)
	}

	modDir := testdata + fileutils.GetFileSeparator() + "mods" + fileutils.GetFileSeparator()
	tests := []struct {
		name                        string
		expectedReplaceDependencies []string
	}{
		{"replaceBlockFirst",
			[]string{
				"        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceBlockLast", []string{
				"        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceLineFirst",
			[]string{
				"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceLineLast",
			[]string{
				"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceBothLineFirst",
			[]string{
				"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1",
				"        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
		{
			"replaceBothBlockFirst", []string{
				"replace github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"replace github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1",
				"        github.com/Masterminds/sprig => github.com/Masterminds/sprig v2.13.0+incompatible",
				"        github.com/Microsoft/ApplicationInsights-Go => github.com/Microsoft/ApplicationInsights-Go v0.3.1"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modContent, err := ioutil.ReadFile(modDir + test.name + ".txt")
			if err != nil {
				t.Error(err)
			}
			replaceLinerDependencies, err := parseModForReplaceDependencies(string(modContent))
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
