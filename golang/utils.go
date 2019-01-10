package golang

import (
	"errors"
	"fmt"
	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
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
			notFoundRegExp, err = initRegExp(`[^go:]([^\/\r\n]+\/[^\r\n\s:]*).*(404 Not Found)`, Error)
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
	if len(pattern.MatchedResults) == 3 {
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