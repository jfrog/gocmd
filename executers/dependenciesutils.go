package executers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	multifilereader "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	FailedToRetrieve          = "Failed to retrieve"
	FromBothArtifactoryAndVcs = "from both Artifactory and VCS"
)

func performHeadRequest(auth auth.ServiceDetails, client *httpclient.HttpClient, targetRepo, module, version string) (*http.Response, error) {
	url := auth.GetUrl() + "api/go/" + targetRepo + "/" + module + "/@v/" + version + ".mod"
	resp, _, err := client.SendHead(url, auth.CreateHttpClientDetails(), "")
	if err != nil {
		return nil, err
	}
	log.Debug("Artifactory head request response for", url, ":", resp.StatusCode)
	return resp, nil
}

// Creating dependency with the mod file in the temp directory
func createDependencyInTemp(zipPath, tempDir string) (err error) {
	multiReader, err := multifilereader.NewMultiFileReaderAt([]string{zipPath})
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = fileutils.Unzip(multiReader, multiReader.Size(), tempDir)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

// Returns the actual path to the dependency.
// If in the path there are capital letters, the Go convention is to use "!" before the letter.
// The letter itself in lowercase.
func goModEncode(name string) string {
	path := ""
	for _, letter := range name {
		if unicode.IsUpper(letter) {
			path += "!" + strings.ToLower(string(letter))
		} else {
			path += string(letter)
		}
	}
	return path
}

// Returns the path to the dependency decoded form lower case to upper case
// If in the path there are capital letters, the Go convention is to use "!" before the letter.
// The letter itself in lowercase. This function will decode back to Upper case
func goModDecode(name string) string {
	var str string
	for i := 0; i < len(name); i++ {
		if string(name[i]) == "!" {
			if i < len(name)-1 {
				r := rune(name[i+1])
				str += string(unicode.ToUpper(r))
				i++
			}
		} else {
			str += string(name[i])
		}
	}
	return str
}

// Downloads the mod file from Artifactory to the Go cache
func downloadModFileFromArtifactoryToLocalCache(cachePath, targetRepo, name, version string, auth auth.ServiceDetails, client *httpclient.HttpClient) string {
	pathToModuleCache := filepath.Join(cachePath, name, "@v")
	dirExists, err := fileutils.IsDirExists(pathToModuleCache, false)
	if err != nil {
		log.Error(fmt.Sprintf("Received an error: %s for %s@%s", err, name, version))
		return ""
	}

	if dirExists {
		url := auth.GetUrl() + "api/go/" + targetRepo + "/" + name + "/@v/" + version + ".mod"
		log.Debug("Downloading mod file from Artifactory:", url)
		downloadFileDetails := &httpclient.DownloadFileDetails{
			FileName: version + ".mod",
			// Artifactory URL
			DownloadPath:  url,
			LocalPath:     pathToModuleCache,
			LocalFileName: version + ".mod",
		}
		resp, err := client.DownloadFile(downloadFileDetails, "", auth.CreateHttpClientDetails(), false)
		if err != nil {
			log.Error(fmt.Sprintf("Received an error %s downloading a file: %s to the local path: %s", err.Error(), downloadFileDetails.FileName, downloadFileDetails.LocalPath))
			return ""
		}

		log.Debug(fmt.Sprintf("Received %d from Artifactory %s", resp.StatusCode, url))
		return filepath.Join(downloadFileDetails.LocalPath, downloadFileDetails.LocalFileName)
	}
	return ""
}

func shouldDownloadFromArtifactory(module, version, targetRepo string, auth auth.ServiceDetails, client *httpclient.HttpClient) (bool, error) {
	res, err := performHeadRequest(auth, client, targetRepo, module, version)
	if err != nil {
		return false, err
	}
	if res.StatusCode == 200 {
		return true, nil
	}
	return false, nil
}

func GetDependencies(cachePath string, moduleSlice map[string]bool) ([]Package, error) {
	var deps []Package
	for module := range moduleSlice {
		moduleInfo := strings.Split(module, "@")
		name := goModEncode(moduleInfo[0])
		dep, err := createDependency(cachePath, name, goModEncode(moduleInfo[1]))
		if err != nil {
			return nil, err
		}
		if dep != nil {
			deps = append(deps, *dep)
		}
	}
	return deps, nil
}

// Creates a go dependency.
// Returns a nil value in case the dependency does not include a zip in the cache.
func createDependency(cachePath, dependencyName, version string) (*Package, error) {
	// We first check if the this dependency has a zip binary in the local go cache.
	// If it does not, nil is returned. This seems to be a bug in go.
	zipPath, err := getPackageZipLocation(cachePath, dependencyName, version)

	if err != nil {
		return nil, err
	}

	if zipPath == "" {
		return nil, nil
	}

	dep := Package{}

	dep.id = strings.Join([]string{dependencyName, version}, ":")
	dep.version = version
	dep.zipPath = zipPath
	dep.modPath = filepath.Join(cachePath, dependencyName, "@v", version+".mod")
	dep.infoPath = filepath.Join(cachePath, dependencyName, "@v", version+".info")
	dep.modContent, err = ioutil.ReadFile(dep.modPath)
	if err != nil {
		return &dep, errorutils.CheckError(err)
	}

	return &dep, nil
}

// Returns the path to the package zip file if exists.
func getPackageZipLocation(cachePath, dependencyName, version string) (string, error) {
	zipPath, err := getPackagePathIfExists(cachePath, dependencyName, version)
	if err != nil {
		return "", err
	}

	if zipPath != "" {
		return zipPath, nil
	}

	zipPath, err = getPackagePathIfExists(filepath.Dir(cachePath), dependencyName, version)

	if err != nil {
		return "", err
	}

	return zipPath, nil
}

// Validates if the package zip file exists.
func getPackagePathIfExists(cachePath, dependencyName, version string) (zipPath string, err error) {
	zipPath = filepath.Join(cachePath, dependencyName, "@v", version+".zip")
	fileExists, err := fileutils.IsFileExists(zipPath, false)
	if err != nil {
		log.Warn(fmt.Sprintf("Could not find zip binary for dependency '%s' at %s.", dependencyName, zipPath))
		return "", err
	}
	// Zip binary does not exist, so we skip it by returning a nil dependency.
	if !fileExists {
		log.Debug("The following file is missing:", zipPath)
		return "", nil
	}
	return zipPath, nil
}

func removeGoSum(path string) error {
	// Remove go.sum file to avoid checksum conflicts with the old go.sum
	goSum := filepath.Join(filepath.Dir(path), "go.sum")
	exists, err := fileutils.IsFileExists(goSum, false)
	if err != nil {
		return err
	}
	if exists {
		err = os.Remove(goSum)
		if errorutils.CheckError(err) != nil {
			return err
		}
	}
	return nil
}

type previousTries struct {
	triedFromArtifactory bool
	triedFromVCS         bool
}

func (pt *previousTries) setTriedFrom(usedProxy bool) {
	if usedProxy {
		pt.triedFromArtifactory = true
	} else {
		pt.triedFromVCS = true
	}
}
