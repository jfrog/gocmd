package utils

import (
	"fmt"
	"github.com/jfrog/gocmd/cache"
	"github.com/jfrog/gocmd/cmd"
	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const GOPROXY = "GOPROXY"

// Returns true if a dependency was not found Artifactory.
func DependencyNotFoundInArtifactory(err error, noRegistry bool) bool {
	regExp, errRegex := utils.GetRegExp(`^404( Not Found)?(\s)?:`)
	if errRegex != nil {
		LogError(errRegex)
		return false
	}
	if !noRegistry && regExp.Match([]byte(err.Error())) {
		return true
	}
	return false
}

func SetGoProxyWithApi(repoName string, details auth.ArtifactoryDetails) error {
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
