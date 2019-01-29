package executers

import (
	"fmt"
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services/go"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
	"strings"
)

func PublishDependencies(goArg, targetRepo string, noRegistry bool, serviceManager *artifactory.ArtifactoryServicesManager) error {
	wd, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	dependenciesInterface := &Package{}
	pd := &publishDependencies{wd: wd, dependenciesInterface: dependenciesInterface, targetRepo: targetRepo, serviceManager: serviceManager, goArg: goArg}
	register(pd)
	return ExecuteGo(goArg, noRegistry, serviceManager)
}

type publishDependencies struct {
	wd                    string
	targetRepo            string
	goArg                 string
	serviceManager        *artifactory.ArtifactoryServicesManager
	dependenciesInterface GoPackage
}

// Resolve artifacts from VCS and publish the missing artifacts to Artifactory
func (pd *publishDependencies) execute() error {
	rootProjectDir, err := cmd.GetProjectRoot()
	if err != nil {
		return err
	}
	cache := cache.DependenciesCache{}
	dependenciesToPublish, err := collectProjectDependencies(pd.targetRepo, rootProjectDir, &cache, pd.serviceManager.GetConfig().GetArtDetails())
	if err != nil || len(dependenciesToPublish) == 0 {
		return err
	}
	err = execute(pd.targetRepo, true, pd.dependenciesInterface, &cache, dependenciesToPublish, pd.serviceManager)
	if err != nil {
		return err
	}
	// Lets run the same command again now that all the dependencies were downloaded.
	// Need to run only if the command is not go mod download and go mod tidy since this was run by the CLI to download and publish to Artifactory
	if pd.goArg != "" && !strings.Contains(pd.goArg, "mod download") && !strings.Contains(pd.goArg, "mod tidy") {
		// Remove the go.sum file, since it includes information which is not up to date (it was created by the "go mod tidy" command executed without Artifactory
		err = removeGoSumFile(pd.wd, rootProjectDir)
		if err != nil {
			log.Error("Received an error removing go sum file:", err)
		}
	}
	return cmd.RunGo(pd.goArg)
}

func (pd *publishDependencies) SetGoProxyEnvVar(details auth.ArtifactoryDetails) error {
	return setGoProxyEnvVar(pd.targetRepo, details)
}

func removeGoSumFile(wd, rootDir string) error {
	log.Debug("Changing back to the working directory", wd)
	err := os.Chdir(wd)
	if err != nil {
		return err
	}

	goSumFile := filepath.Join(rootDir, "go.sum")
	exists, err := fileutils.IsFileExists(goSumFile, false)
	if err != nil {
		return err
	}
	if exists {
		return errorutils.CheckError(os.Remove(goSumFile))
	}
	return nil
}

// Represent go dependency package.
type Package struct {
	buildInfoDependencies []buildinfo.Dependency
	id                    string
	modContent            []byte
	zipPath               string
	modPath               string
	version               string
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
