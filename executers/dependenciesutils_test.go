package executers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	err = createDependencyInTemp(dep.GetZipPath(), tempDirPath)
	if err != nil {
		t.Error(err)
	}
	modPath := getModPathInTemp(dep.id, tempDirPath)
	path := filepath.Join(tempDirPath, "github.com", "test@v1.2.3", "go.mod")
	if path != modPath {
		t.Error(fmt.Sprintf("Expected %s, got %s", path, modPath))
	}

	err = os.RemoveAll(filepath.Join(tempDirPath, "github.com"))
	if err != nil {
		t.Error(err)
	}
}

func getModPathInTemp(moduleId, tempDir string) string {
	moduleInfo := strings.Split(moduleId, ":")
	moduleInfo[0] = goModDecode(moduleInfo[0])
	if len(moduleInfo) > 1 {
		moduleInfo[1] = goModDecode(moduleInfo[1])
	}
	moduleId = strings.Join(moduleInfo, ":")
	modulePath := strings.Replace(moduleId, ":", "@", 1)
	path := filepath.Join(tempDir, modulePath, "go.mod")
	return path
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
