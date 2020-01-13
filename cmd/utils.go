package cmd

import (
	"errors"
	"fmt"
	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

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
		protocolRegExp, err = initRegExp(utils.CredentialsInUrlRegexp, MaskCredentials)
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

// Mask the credentials information from the line.
func MaskCredentials(pattern *gofrogio.CmdOutputPattern) (string, error) {
	return utils.MaskCredentials(pattern.Line, pattern.MatchedResults[0]), nil
}

func Error(pattern *gofrogio.CmdOutputPattern) (string, error) {
	_, err := fmt.Fprint(os.Stderr, pattern.Line)
	if err != nil {
		return "", errorutils.WrapError(err)
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
	if errorutils.WrapError(err) != nil {
		return
	}
	modFileContent, err = ioutil.ReadFile(filePath)
	errorutils.WrapError(err)
	return
}

func outputToMap(output string) map[string]bool {
	lineOutput := strings.Split(output, "\n")
	var result []string
	mapOfDeps := map[string]bool{}
	for _, line := range lineOutput {
		splitLine := strings.Split(line, " ")
		if len(splitLine) == 2 {
			mapOfDeps[splitLine[1]] = true
			result = append(result, splitLine[1])
		}
	}
	return mapOfDeps
}
