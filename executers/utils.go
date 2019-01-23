package executers

import (
	"errors"
	"fmt"
	"github.com/jfrog/gocmd/dependencies"
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"strings"
)

func execute(targetRepo string, failOnError bool, dependenciesInterface dependencies.GoPackage, cache *cache.DependenciesCache, dependenciesToPublish map[string]bool, serviceManager *artifactory.ArtifactoryServicesManager) error {
	cachePath, packageDependencies, err := getDependencies(dependenciesToPublish)
	if err != nil {
		if failOnError {
			return err
		}
		log.Error("Received an error retrieving project dependencies:", err)
	}
	err = populateAndPublish(targetRepo, cachePath, dependenciesInterface, packageDependencies, cache, serviceManager)
	if err != nil {
		if failOnError {
			return err
		}
		log.Error("Received an error populating and publishing the dependencies:", err)
	}
	logFinishedMsg(cache)
	return nil
}

func logFinishedMsg(cache *cache.DependenciesCache) {
	log.Info(fmt.Sprintf("Done building and publishing %d go dependencies to Artifactory out of a total of %d dependencies.", cache.GetSuccesses(), cache.GetTotal()))

}
func collectDependencies(targetRepo, rootProjectDir string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) (dependenciesToPublish map[string]bool, err error) {
	dependenciesToPublish, err = dependencies.CollectProjectDependencies(targetRepo, rootProjectDir, cache, serviceManager.GetConfig().GetArtDetails())
	return
}

// Download the dependencies from VCS and publish them to Artifactory.
func getDependencies(dependenciesToPublish map[string]bool) (cachePath string, packageDependencies []dependencies.Package, err error) {
	cachePath, err = dependencies.GetCachePath()
	if err != nil {
		return
	}
	packageDependencies, err = dependencies.GetDependencies(cachePath, dependenciesToPublish)
	return
}

func populateAndPublish(targetRepo, cachePath string, dependenciesInterface dependencies.GoPackage, packageDependencies []dependencies.Package, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	cache.IncrementTotal(len(packageDependencies))
	for _, dep := range packageDependencies {
		dependenciesInterface = dependenciesInterface.New(cachePath, dep)
		err := dependenciesInterface.PopulateModAndPublish(targetRepo, cache, serviceManager)
		if err != nil {
			// If using recursive tidy - the error always nil. If we got here, means that this error happened when not using the recursive tidy flag.
			return err
		}
	}
	return nil
}

func ExecuteGo(goArg, targetRepo string, noRegistry bool, serviceManager *artifactory.ArtifactoryServicesManager) error {
	if !noRegistry {
		artDetails := serviceManager.GetConfig().GetArtDetails()
		err := cmd.SetGoProxyEnvVar(artDetails.GetUrl(), artDetails.GetUser(), artDetails.GetPassword(), targetRepo)
		if err != nil {
			return err
		}
	}

	err := cmd.RunGo(goArg)

	if err != nil {
		if dependencyNotFoundInArtifactory(err, noRegistry) {
			log.Info("Received", err.Error(), "from Artifactory. Trying download the dependencies from the VCS...")
			err = DownloadFromVcsAndPublishIfRequired()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// Returns true if a dependency was not found Artifactory.
func dependencyNotFoundInArtifactory(err error, noRegistry bool) bool {
	if !noRegistry && strings.Contains(err.Error(), "404 Not Found") {
		return true
	}
	return false
}

// Downloads all dependencies from VCS and publish them to Artifactory.
func DownloadFromVcsAndPublishIfRequired() error {
	err := os.Unsetenv(cmd.GOPROXY)
	if err != nil {
		return err
	}

	executor := GetCompatibleExecutor()
	if executor != nil {
		return executor.execute()
	}

	return errorutils.CheckError(errors.New("No executors were registered."))
}