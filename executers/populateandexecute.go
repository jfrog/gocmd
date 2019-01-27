package executers

import (
	"github.com/jfrog/gocmd/dependencies"
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

func DownloadFromVcsWithPopulation(targetRepo, goModEditMessage string, serviceManager *artifactory.ArtifactoryServicesManager) error {
	dependenciesInterface := &dependencies.PackageWithDeps{GoModEditMessage: goModEditMessage}
	err := dependenciesInterface.Init()
	if err != nil {
		return err
	}
	register(&populateAndExecute{dependenciesInterface:dependenciesInterface, targetRepo:targetRepo, serviceManager:serviceManager})

	return unsetGoProxyAndExecute()
}

type populateAndExecute struct {
	targetRepo string
	serviceManager *artifactory.ArtifactoryServicesManager
	dependenciesInterface dependencies.GoPackage
}

// Populates and publish the dependencies.
func (pae *populateAndExecute) execute() error {
	err := fileutils.CreateTempDirPath()
	if err != nil {
		return err
	}
	defer fileutils.RemoveTempDir()
	rootProjectDir, err := cmd.GetProjectRoot()
	if err != nil {
		return err
	}
	cache := cache.DependenciesCache{}
	dependenciesToPublish, err := collectDependencies(pae.targetRepo, rootProjectDir, &cache, pae.serviceManager)
	if err != nil || len(dependenciesToPublish) == 0 {
		return err
	}
	execute(pae.targetRepo, false, pae.dependenciesInterface, &cache, dependenciesToPublish, pae.serviceManager)
	return nil
}
