package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/jfrog/gocmd/cache"
	"github.com/jfrog/gocmd/cmd"
	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const GOPROXY = "GOPROXY"

func GetArtifactoryApiUrl(repoName string, details auth.ServiceDetails) (string, error) {
	rtUrl, err := url.Parse(details.GetUrl())
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	username := details.GetUser()
	password := details.GetPassword()

	// Get credentials from access-token if exists.
	if details.GetAccessToken() != "" {
		log.Debug("Using proxy with access-token.")
		username, err = auth.ExtractUsernameFromAccessToken(details.GetAccessToken())
		if err != nil {
			return "", err
		}
		password = details.GetAccessToken()
	}

	if username != "" && password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}
	rtUrl.Path += "api/go/" + repoName
	return rtUrl.String(), nil
}

// GetPackageVersion returns the matching version for the packageName string using the Artifactory details that are provided.
// PackageName string should be in the following format: <Package Path>/@V/<Requested Branch Name>.info OR latest.info
// For example the jfrog/jfrog-cli/@v/master.info packageName will return the corresponding canonical version (vX.Y.Z) string for the jfrog-cli master branch.
func GetPackageVersion(repoName, packageName string, details auth.ServiceDetails) (string, error) {
	artifactoryApiUrl, err := GetArtifactoryApiUrl(repoName, details)
	if err != nil {
		return "", err
	}
	artHttpDetails := details.CreateHttpClientDetails()
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return "", err
	}
	artifactoryApiUrl = artifactoryApiUrl + "/" + packageName
	resp, body, _, err := client.SendGet(artifactoryApiUrl, true, artHttpDetails, "")
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errorutils.CheckError(errors.New("Artifactory response: " + resp.Status))
	}
	// Extract version from response
	var version PackageVersionResponseContent
	err = json.Unmarshal(body, &version)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return version.Version, nil
}

// GetCachePath returns the location of downloads dir insied the GOMODCACHE
func GetCachePath() (string, error) {
	goModCachePath, err := GetGoModCachePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(goModCachePath, "cache", "download"), nil
}

// GetGoModCachePath returns the location of the go module cache
func GetGoModCachePath() (string, error) {
	goPath, err := getGOPATH()
	if err != nil {
		return "", err
	}
	return filepath.Join(goPath, "pkg", "mod"), nil
}

// GetGOPATH returns the location of the GOPATH
func getGOPATH() (string, error) {
	goCmd, err := cmd.NewCmd()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	goCmd.Command = []string{"env", "GOPATH"}
	output, err := gofrogio.RunCmdOutput(goCmd)
	if errorutils.CheckError(err) != nil {
		return "", fmt.Errorf("Could not find GOPATH env: %s", err.Error())
	}
	return strings.TrimSpace(parseGoPath(string(output))), nil
}

func GetRegex() (regExp *RegExp, err error) {
	emptyRegex, err := utils.GetRegExp(`^\s*require (?:[\(\w\.@:%_\+-.~#?&]?.+)`)
	if err != nil {
		return
	}

	indirectRegex, err := utils.GetRegExp(`(// indirect)$`)
	if err != nil {
		return
	}

	regExp = &RegExp{
		notEmptyModRegex: emptyRegex,
		indirectRegex:    indirectRegex,
	}
	return
}

func LogError(err error) {
	if err != nil {
		log.Error(err)
	}
}

func LogDebug(err error, usedProxy bool) {
	message := "Received " + err.Error() + " from"
	if usedProxy {
		message += " Artifactory."
	} else {
		message += " VCS."
	}
	log.Warn(message)
}

func LogFinishedMsg(cache *cache.DependenciesCache) {
	log.Info(fmt.Sprintf("Done building and publishing %d go dependencies to Artifactory out of a total of %d dependencies.", cache.GetSuccesses(), cache.GetTotal()))
}

type RegExp struct {
	notEmptyModRegex *regexp.Regexp
	indirectRegex    *regexp.Regexp
}

func (reg *RegExp) GetNotEmptyModRegex() *regexp.Regexp {
	return reg.notEmptyModRegex
}

func (reg *RegExp) GetIndirectRegex() *regexp.Regexp {
	return reg.indirectRegex
}

func parseGoPath(goPath string) string {
	if runtime.GOOS == "windows" {
		goPathSlice := strings.Split(goPath, ";")
		return goPathSlice[0]
	}
	goPathSlice := strings.Split(goPath, ":")
	return goPathSlice[0]
}

type PackageVersionResponseContent struct {
	Version string `json:"Version,omitempty"`
}
