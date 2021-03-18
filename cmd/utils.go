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
const minimumGoVersion = "go1.13"

// Never use this value, use shouldMaskPassword().
var shouldMask *bool = nil

func prepareRegExp() error {
	err := prepareGlobalRegExp()
	if err != nil {
		return err
	}
	return prepareNotFoundZipRegExp()
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

	if notFoundRegExp == nil {
		log.Debug("Initializing not found regexp")
		notFoundRegExp, err = initRegExp(`^go: ([^\/\r\n]+\/[^\r\n\s:]*).*(404( Not Found)?[\s]?)$`, Error)
		if err != nil {
			return err
		}
	}

	if notFoundGo113RegExp == nil {
		log.Debug("Initializing not found go 1.13 regexp")
		notFoundGo113RegExp, err = initRegExp(`^[\s]*[\s](.+)@(.+):[\s]reading[\s].*(404( Not Found)?[\s]?)$`, Error)
		if err != nil {
			return err
		}
	}

	if unrecognizedImportRegExp == nil {
		log.Debug("Initializing unrecognized import path regexp")
		unrecognizedImportRegExp, err = initRegExp(`[^go:]([^\/\r\n]+\/[^\r\n\s:]*).*(unrecognized import path)`, Error)
		if err != nil {
			return err
		}
	}

	if unknownRevisionRegExp == nil {
		log.Debug("Initializing unknown revision regexp")
		unknownRevisionRegExp, err = initRegExp(`[^go:]([^\/\r\n]+\/[^\r\n\s:]*).*(unknown revision)`, Error)
	}

	return err
}

func prepareNotFoundZipRegExp() error {
	var err error
	if notFoundZipRegExp == nil {
		log.Debug("Initializing not found zip file")
		notFoundZipRegExp, err = initRegExp(`unknown import path ["]([^\/\r\n]+\/[^\r\n\s:]*)["].*(404( Not Found)?[\s]?)$`, Error)
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

func outputToMap(output string) map[string]bool {
	lineOutput := strings.Split(output, "\n")
	mapOfDeps := map[string]bool{}
	lines := len(lineOutput)
	i := 0
	// Ignore all 'go' messages
	for ; i < lines; i++ {
		splitLine := strings.Split(lineOutput[i], " ")
		if splitLine[0] != "go:" {
			break
		}
	}
	// First dependency in the list  is the module itself
	i++
	for ; i < lines; i++ {
		splitLine := strings.Split(lineOutput[i], " ")
		splitLineLen := len(splitLine)
		if splitLineLen == 2 {
			mapOfDeps[splitLine[0]+"@"+splitLine[1]] = true
			continue
		}
		// Check if there is a "replace" statement
		if splitLineLen >= 3 {
			if splitLine[2] == "=>" {
				if splitLineLen == 5 {
					mapOfDeps[splitLine[3]+"@"+splitLine[4]] = true
					continue
				} else {
					mapOfDeps[splitLine[3]] = true
				}
			}
		}
	}
	return mapOfDeps
}

// Go performs password redaction from url since version 1.13.
// Only if go version before 1.13, should manually perform password masking.
func shouldMaskPassword() (bool, error) {
	if shouldMask == nil {
		goVersion, err := getParsedGoVersion()
		if err != nil {
			return false, err
		}
		shouldMaskBool := !goVersion.AtLeast(minimumGoVersion)
		shouldMask = &shouldMaskBool
	}

	return *shouldMask, nil
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
