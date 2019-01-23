package dependencies

import (
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"regexp"
)

type GoPackage interface {
	PopulateModAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	Init() error
	prepareAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	New(cachePath string, dependency Package) GoPackage
}

type RegExp struct {
	notEmptyModRegex *regexp.Regexp
	indirectRegex    *regexp.Regexp
	generatedBy      *regexp.Regexp
}

func (reg *RegExp) GetNotEmptyModRegex() *regexp.Regexp {
	return reg.notEmptyModRegex
}

func (reg *RegExp) GetIndirectRegex() *regexp.Regexp {
	return reg.indirectRegex
}

func (reg *RegExp) GetGeneratedBy() *regexp.Regexp {
	return reg.generatedBy
}