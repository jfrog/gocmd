package dependencies

import (
	"github.com/jfrog/gocmd/golang"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"regexp"
)

type GoPackage interface {
	PopulateModAndPublish(targetRepo string, cache *golang.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	Init() error
	prepareAndPublish(targetRepo string, cache *golang.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	New(cachePath string, dependency Package) GoPackage
}

type RegExp struct {
	notEmptyModRegex *regexp.Regexp
	indirectRegex    *regexp.Regexp
	editedByJFrogCli *regexp.Regexp
}

func (reg *RegExp) GetNotEmptyModRegex() *regexp.Regexp {
	return reg.notEmptyModRegex
}

func (reg *RegExp) GetIndirectRegex() *regexp.Regexp {
	return reg.indirectRegex
}

func (reg *RegExp) GetEditedByJFrogCli() *regexp.Regexp {
	return reg.editedByJFrogCli
}