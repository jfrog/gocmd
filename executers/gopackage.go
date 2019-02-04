package executers

import (
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/jfrog-client-go/artifactory"
)

type GoPackage interface {
	PopulateModAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	Init() error
	prepareAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	New(cachePath string, dependency Package) GoPackage
}
