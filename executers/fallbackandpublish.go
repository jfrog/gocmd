package executers

import (
	"fmt"
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services/go"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

// Represent go dependency package.
type Package struct {
	buildInfoDependencies []buildinfo.Dependency
	id                    string
	modContent            []byte
	zipPath               string
	modPath               string
	version               string
}

// Runs Go, with multiple fallbacks if needed and publish missing dependencies to Artifactory
func RunGoFallbackAndPublish(goArg, targetRepo string, noRegistry bool, serviceManager *artifactory.ArtifactoryServicesManager) error {
	if !noRegistry {
		artDetails := serviceManager.GetConfig().GetArtDetails()
		err := setGoProxyWithApi(targetRepo, artDetails)
		if err != nil {
			return err
		}
	}

	err := cmd.RunGo(goArg)

	if err != nil {
		if dependencyNotFoundInArtifactory(err, noRegistry) {
			log.Info("Received", err.Error(), "from Artifactory. Trying to download dependencies from VCS...")
			err := os.Unsetenv(GOPROXY)
			if err != nil {
				return err
			}

			err = collectDependenciesAndPublish(targetRepo, true,  &Package{}, serviceManager)
			if err != nil {
				return err
			}
			return cmd.RunGo(goArg)
		} else {
			return err
		}
	}
	return nil
}

func (dependencyPackage *Package) New(cachePath string, dep Package) GoPackage {
	dependencyPackage.modContent = dep.modContent
	dependencyPackage.zipPath = dep.zipPath
	dependencyPackage.version = dep.version
	dependencyPackage.id = dep.id
	dependencyPackage.buildInfoDependencies = dep.buildInfoDependencies
	dependencyPackage.modPath = dep.modPath
	return dependencyPackage
}

func (dependencyPackage *Package) GetId() string {
	return dependencyPackage.id
}

func (dependencyPackage *Package) GetModContent() []byte {
	return dependencyPackage.modContent
}

func (dependencyPackage *Package) SetModContent(modContent []byte) {
	dependencyPackage.modContent = modContent
}

func (dependencyPackage *Package) GetZipPath() string {
	return dependencyPackage.zipPath
}

// Init the dependency information if needed.
func (dependencyPackage *Package) Init() error {
	return nil
}

func (dependencyPackage *Package) PopulateModAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	published, _ := cache.GetMap()[dependencyPackage.GetId()]
	if !published {
		return dependencyPackage.prepareAndPublish(targetRepo, cache, serviceManager)
	} else {
		log.Debug(fmt.Sprintf("Dependency %s was published previosly to Artifactory", dependencyPackage.GetId()))
	}
	return nil
}

// Prepare for publishing and publish the dependency to Artifactory
func (dependencyPackage *Package) prepareAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	successOutOfTotal := fmt.Sprintf("%d/%d", cache.GetSuccesses()+1, cache.GetTotal())
	err := dependencyPackage.Publish(successOutOfTotal, targetRepo, serviceManager)
	if err != nil {
		cache.IncrementFailures()
		return err
	}
	cache.IncrementSuccess()
	return nil
}

func (dependencyPackage *Package) Publish(summary string, targetRepo string, servicesManager *artifactory.ArtifactoryServicesManager) error {
	message := fmt.Sprintf("Publishing: %s to %s", dependencyPackage.id, targetRepo)
	if summary != "" {
		message += ":" + summary
	}
	log.Info(message)
	params := _go.NewGoParams()
	params.ZipPath = dependencyPackage.zipPath
	params.ModContent = dependencyPackage.modContent
	params.Version = dependencyPackage.version
	params.TargetRepo = targetRepo
	params.ModuleId = dependencyPackage.id
	params.ModPath = dependencyPackage.modPath

	return servicesManager.PublishGoProject(params)
}

func (dependencyPackage *Package) Dependencies() []buildinfo.Dependency {
	return dependencyPackage.buildInfoDependencies
}
