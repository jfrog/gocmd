package cmd

import (
	"errors"
	"fmt"
	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Compiles all the regex once
func prepareCmdOutputPattern() error {
	var err error
	if protocolRegExp == nil {
		log.Debug("Initializing protocol regexp")
		protocolRegExp, err = initRegExp(`((http|https):\/\/\w.*?:\w.*?@)`, MaskCredentials)
		if err != nil {
			return err
		}
	}
	if notFoundRegExp == nil || unrecognizedImportRegExp == nil || unknownRevisionRegExp == nil {
		if notFoundRegExp == nil {
			log.Debug("Initializing not found regexp")
			notFoundRegExp, err = initRegExp(`^go: ([^\/\r\n]+\/[^\r\n\s:]*).*(404( Not Found)?[\s]?)$`, Error)
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
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func initRegExp(regex string, execFunc func(pattern *gofrogio.CmdOutputPattern) (string, error)) (*gofrogio.CmdOutputPattern, error) {
	regExp, err := GetRegExp(regex)
	if err != nil {
		return &gofrogio.CmdOutputPattern{}, err
	}

	outputPattern := &gofrogio.CmdOutputPattern{
		RegExp: regExp,
	}

	outputPattern.ExecFunc = execFunc
	return outputPattern, nil
}

// Mask the credentials information from the line. The credentials are build as user:password
// For example: http://user:password@127.0.0.1:8081/artifactory/path/to/repo
func MaskCredentials(pattern *gofrogio.CmdOutputPattern) (string, error) {
	splittedResult := strings.Split(pattern.MatchedResults[0], "//")
	return strings.Replace(pattern.Line, pattern.MatchedResults[0], splittedResult[0]+"//***.***@", 1), nil
}

func Error(pattern *gofrogio.CmdOutputPattern) (string, error) {
	fmt.Fprintf(os.Stderr, pattern.Line)
	if len(pattern.MatchedResults) >= 3 {
		return "", errors.New(pattern.MatchedResults[2] + ":" + strings.TrimSpace(pattern.MatchedResults[1]))
	}
	return "", errors.New(fmt.Sprintf("Regex found the following values: %s", pattern.MatchedResults))
}

func GetRegExp(regex string) (*regexp.Regexp, error) {
	regExp, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}

	return regExp, nil
}

func GetSumContentAndRemove(rootProjectDir string) (sumFileContent []byte, sumFileStat os.FileInfo, err error) {
	sumFileExists, err := fileutils.IsFileExists(filepath.Join(rootProjectDir, "go.sum"), false)
	if err != nil {
		return
	}
	if sumFileExists {
		log.Debug("Sum file exists:", rootProjectDir)
		sumFileContent, sumFileStat, err = GetFileDetails(filepath.Join(rootProjectDir, "go.sum"))
		if err != nil {
			return
		}
		log.Debug("Removing file:", filepath.Join(rootProjectDir, "go.sum"))
		err = os.Remove(filepath.Join(rootProjectDir, "go.sum"))
		if err != nil {
			return
		}
		return
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
