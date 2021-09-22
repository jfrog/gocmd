package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

// Minimum go version, which its output does not require to mask passwords in URLs.
const minGoVersionForMasking = "go1.13"

// Max go version, which automatically modify go.mod and go.sum when executing build commands.
const maxGoVersionAutomaticallyModifyMod = "go1.15"

// Never use this value, use shouldMaskPassword().
var shouldMask *bool = nil

// Never use this value, use automaticallyModifyMod().
var autoModify *bool = nil

func prepareRegExp() error {
	return prepareGlobalRegExp()
}

// Compiles all the regex once
func prepareGlobalRegExp() error {
	var err error
	if protocolRegExp == nil {
		log.Debug("Initializing protocol regexp")
		protocolRegExp, err = initRegExp(utils.CredentialsInUrlRegexp, RemoveCredentials)
		if err != nil {
			return err
		}
	}

	return err
}

func initRegExp(regex string, execFunc func(pattern *gofrogio.CmdOutputPattern) (string, error)) (*gofrogio.CmdOutputPattern, error) {
	regExp, err := utils.GetRegExp(regex)
	if err != nil {
		return &gofrogio.CmdOutputPattern{}, err
	}

	outputPattern := &gofrogio.CmdOutputPattern{
		RegExp: regExp,
	}

	outputPattern.ExecFunc = execFunc
	return outputPattern, nil
}

// Remove the credentials information from the line.
func RemoveCredentials(pattern *gofrogio.CmdOutputPattern) (string, error) {
	return utils.RemoveCredentials(pattern.Line, pattern.MatchedResults[0]), nil
}

func Error(pattern *gofrogio.CmdOutputPattern) (string, error) {
	_, err := fmt.Fprint(os.Stderr, pattern.Line)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	if len(pattern.MatchedResults) >= 3 {
		return "", errors.New(pattern.MatchedResults[2] + ":" + strings.TrimSpace(pattern.MatchedResults[1]))
	}
	return "", errors.New(fmt.Sprintf("Regex found the following values: %s", pattern.MatchedResults))
}

func GetGoSum(rootProjectDir string) (sumFileContent []byte, sumFileStat os.FileInfo, err error) {
	sumFileExists, err := fileutils.IsFileExists(filepath.Join(rootProjectDir, "go.sum"), false)
	if err == nil && sumFileExists {
		log.Debug("Sum file exists:", rootProjectDir)
		sumFileContent, sumFileStat, err = GetFileDetails(filepath.Join(rootProjectDir, "go.sum"))
	}
	return
}

func RestoreSumFile(rootProjectDir string, sumFileContent []byte, sumFileStat os.FileInfo) error {
	log.Debug("Restoring file:", filepath.Join(rootProjectDir, "go.sum"))
	err := ioutil.WriteFile(filepath.Join(rootProjectDir, "go.sum"), sumFileContent, sumFileStat.Mode())
	if err != nil {
		return err
	}
	return nil
}

func GetFileDetails(filePath string) (modFileContent []byte, modFileStat os.FileInfo, err error) {
	modFileStat, err = os.Stat(filePath)
	if errorutils.CheckError(err) != nil {
		return
	}
	modFileContent, err = ioutil.ReadFile(filePath)
	errorutils.CheckError(err)
	return
}

func listToMap(output string) map[string]bool {
	lineOutput := strings.Split(output, "\n")
	mapOfDeps := map[string]bool{}
	for _, line := range lineOutput {
		// The expected syntax : github.com/name@v1.2.3
		if len(strings.Split(line, "@")) == 2 && mapOfDeps[line] == false {
			mapOfDeps[line] = true
			continue
		}
	}
	return mapOfDeps
}

func graphToMap(output string) map[string][]string {
	lineOutput := strings.Split(output, "\n")
	mapOfDeps := map[string][]string{}
	for _, line := range lineOutput {
		// The expected syntax : github.com/parentname@v1.2.3 github.com/childname@v1.2.3
		line = strings.ReplaceAll(line, "@v", ":")
		splitLine := strings.Split(line, " ")
		if len(splitLine) == 2 {
			parent := splitLine[0]
			child := splitLine[1]
			mapOfDeps[parent] = append(mapOfDeps[parent], child)
		}
	}
	return mapOfDeps
}

// Go performs password redaction from url since version 1.13.
// Only if go version before 1.13, should manually perform password masking.
func shouldMaskPassword() (bool, error) {
	return compareSpecificVersionToCurVersion(shouldMask, minGoVersionForMasking)
}

// Since version go1.16 build commands (like go build and go list) no longer modify go.mod and go.sum by default.
func automaticallyModifyMod() (bool, error) {
	return compareSpecificVersionToCurVersion(autoModify, maxGoVersionAutomaticallyModifyMod)
}

func compareSpecificVersionToCurVersion(result *bool, comparedVersion string) (bool, error) {
	if result == nil {
		goVersion, err := getParsedGoVersion()
		if err != nil {
			return false, err
		}
		autoModifyBool := !goVersion.AtLeast(comparedVersion)
		result = &autoModifyBool
	}

	return *result, nil
}

func getParsedGoVersion() (*version.Version, error) {
	output, err := GetGoVersion()
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	// Go version output pattern is: 'go version go1.14.1 darwin/amd64'
	// Thus should take the third element.
	splitOutput := strings.Split(output, " ")
	return version.NewVersion(splitOutput[2]), nil
}
