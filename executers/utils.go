package executers

import (
	"fmt"
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/gocmd/utils/cmd"
	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const GOPROXY = "GOPROXY"

// Resolve artifacts from VCS and publish the missing artifacts to Artifactory
func collectDependenciesAndPublish(targetRepo string, failOnError bool, dependenciesInterface GoPackage, serviceManager *artifactory.ArtifactoryServicesManager) error {
	rootProjectDir, err := cmd.GetProjectRoot()
	if err != nil {
		return err
	}
	cache := cache.DependenciesCache{}
	dependenciesToPublish, err := collectProjectDependencies(targetRepo, rootProjectDir, &cache, serviceManager.GetConfig().GetArtDetails())
	if err != nil || len(dependenciesToPublish) == 0 {
		return err
	}
	cachePath, packageDependencies, err := getDependencies(dependenciesToPublish)
	if err != nil {
		if failOnError {
			return err
		}
		log.Error("Received an error retrieving project dependencies:", err)
	}
	err = populateAndPublish(targetRepo, cachePath, dependenciesInterface, packageDependencies, &cache, serviceManager)
	if err != nil {
		return err
	}
	logFinishedMsg(&cache)
	return nil
}

func populateAndPublish(targetRepo, cachePath string, dependenciesInterface GoPackage, packageDependencies []Package, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	cache.IncrementTotal(len(packageDependencies))
	for _, dep := range packageDependencies {
		dependenciesInterface = dependenciesInterface.New(cachePath, dep)
		err := dependenciesInterface.PopulateModAndPublish(targetRepo, cache, serviceManager)
		if err != nil {
			// If using recursive publish - the error always nil. If we got here, means that this error happened when not using recursive publish.
			return err
		}
	}
	return nil
}

// Returns true if a dependency was not found Artifactory.
func dependencyNotFoundInArtifactory(err error, noRegistry bool) bool {
	regExp, errRegex := cmd.GetRegExp(`^404( Not Found)?(\s)?:`)
	if errRegex != nil {
		logError(errRegex)
		return false
	}
	if !noRegistry && regExp.Match([]byte(err.Error())) {
		return true
	}
	return false
}

func setGoProxyWithApi(repoName string, details auth.ArtifactoryDetails) error {
	rtUrl, err := url.Parse(details.GetUrl())
	if err != nil {
		return errorutils.CheckError(err)
	}
	username := details.GetUser()
	password := details.GetPassword()
	if username != "" && password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}
	rtUrl.Path += "api/go/" + repoName
	err = os.Setenv(GOPROXY, rtUrl.String())
	return errorutils.CheckError(err)
}

func GetCachePath() (string, error) {
	goPath, err := getGOPATH()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return filepath.Join(goPath, "pkg", "mod", "cache", "download"), nil
}

func getGOPATH() (string, error) {
	goCmd, err := cmd.NewCmd()
	if err != nil {
		return "", err
	}
	goCmd.Command = []string{"env", "GOPATH"}
	output, err := gofrogio.RunCmdOutput(goCmd)
	if errorutils.CheckError(err) != nil {
		return "", fmt.Errorf("Could not find GOPATH env: %s", err.Error())
	}
	return strings.TrimSpace(string(output)), nil
}

func GetRegex() (regExp *RegExp, err error) {
	emptyRegex, err := cmd.GetRegExp(`^\s*require (?:[\(\w\.@:%_\+-.~#?&]?.+)`)
	if err != nil {
		return
	}

	indirectRegex, err := cmd.GetRegExp(`(// indirect)$`)
	if err != nil {
		return
	}

	regExp = &RegExp{
		notEmptyModRegex: emptyRegex,
		indirectRegex:    indirectRegex,
	}
	return
}

func logError(err error) {
	if err != nil {
		log.Error("Received an error:", err)
	}
}

func logDebug(err error, usedProxy bool) {
	message := "Received " + err.Error() + " from"
	if usedProxy {
		message += " Artifactory."
	} else {
		message += " VCS."
	}
	log.Warn(message)
}

func logFinishedMsg(cache *cache.DependenciesCache) {
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
