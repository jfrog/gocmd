package executers

import (
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"

	"github.com/jfrog/gocmd/cache"
	"github.com/jfrog/jfrog-client-go/artifactory"
	_go "github.com/jfrog/jfrog-client-go/artifactory/services/go"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type GoPackage interface {
	PopulateModAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager artifactory.ArtifactoryServicesManager) error
	Init() error
	prepareAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager artifactory.ArtifactoryServicesManager) error
	New(cachePath string, dependency Package) GoPackage
}

// Represent go dependency package.
type Package struct {
	buildInfoDependencies []buildinfo.Dependency
	id                    string
	modContent            []byte
	zipPath               string
	modPath               string
	infoPath              string
	version               string
}

func (dependencyPackage *Package) New(cachePath string, dep Package) GoPackage {
	dependencyPackage.modContent = dep.modContent
	dependencyPackage.zipPath = dep.zipPath
	dependencyPackage.version = dep.version
	dependencyPackage.id = dep.id
	dependencyPackage.buildInfoDependencies = dep.buildInfoDependencies
	dependencyPackage.modPath = dep.modPath
	dependencyPackage.infoPath = dep.infoPath
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

func (dependencyPackage *Package) PopulateModAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager artifactory.ArtifactoryServicesManager) error {
	published, _ := cache.GetMap()[dependencyPackage.GetId()]
	if !published {
		return dependencyPackage.prepareAndPublish(targetRepo, cache, serviceManager)
	} else {
		log.Debug(fmt.Sprintf("Dependency %s was published previosly to Artifactory", dependencyPackage.GetId()))
	}
	return nil
}

// Prepare for publishing and publish the dependency to Artifactory
func (dependencyPackage *Package) prepareAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager artifactory.ArtifactoryServicesManager) error {
	successOutOfTotal := fmt.Sprintf("%d/%d", cache.GetSuccesses()+1, cache.GetTotal())
	err := dependencyPackage.Publish(successOutOfTotal, targetRepo, serviceManager)
	if err != nil {
		cache.IncrementFailures()
		return err
	}
	cache.IncrementSuccess()
	return nil
}

func (dependencyPackage *Package) Publish(summary string, targetRepo string, servicesManager artifactory.ArtifactoryServicesManager) error {
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
	params.InfoPath = dependencyPackage.infoPath
	_, err := servicesManager.PublishGoProject(params)
	return err
}

func (dependencyPackage *Package) Dependencies() []buildinfo.Dependency {
	return dependencyPackage.buildInfoDependencies
}

// PopulateZip adds the zip file as build-info dependency
func (dependencyPackage *Package) PopulateZip() error {
	// Zip file dependency for the build-info
	zipDependency := buildinfo.Dependency{Id: dependencyPackage.id}
	fileDetails, err := fileutils.GetFileDetails(dependencyPackage.zipPath, true)
	if err != nil {
		return err
	}
	zipDependency.Type = "zip"
	zipDependency.Checksum = &buildinfo.Checksum{Sha1: fileDetails.Checksum.Sha1, Md5: fileDetails.Checksum.Md5}
	dependencyPackage.buildInfoDependencies = append(dependencyPackage.buildInfoDependencies, zipDependency)
	return nil
}
