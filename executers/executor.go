package executers

import (
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
)

var registeredExecutor *GoExecutor

type GoExecutor interface {
	execute() error
	SetGoProxyEnvVar(details auth.ArtifactoryDetails) error
}

func register(executor GoExecutor) {
	registeredExecutor = &executor
}

func GetCompatibleExecutor() GoExecutor {
	return *registeredExecutor
}

type GoPackage interface {
	PopulateModAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	Init() error
	prepareAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error
	New(cachePath string, dependency Package) GoPackage
}
