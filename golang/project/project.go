package project

import (
	"fmt"
	"github.com/jfrog/gocmd/golang"
	"github.com/jfrog/gocmd/golang/project/dependencies"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
	"strings"
)

// Downloads all dependencies from VCS and publish them to Artifactory.
func DownloadFromVcsAndPublish(targetRepo, goArg string, tidyEnum golang.TidyEnum, serviceManager *artifactory.ArtifactoryServicesManager) error {
	wd, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	rootProjectDir, err := golang.GetProjectRoot()
	if err != nil {
		return err
	}
	err = os.Unsetenv(golang.GOPROXY)
	if err != nil {
		return err
	}
	if tidyEnum.GetTidyValue() != "" {
		// Need to run Go without Artifactory to resolve all dependencies.
		cache := golang.DependenciesCache{}
		dependenciesToPublish, err := dependencies.CollectProjectDependencies(targetRepo, &cache, serviceManager.GetConfig().GetArtDetails())
		if err != nil || len(dependenciesToPublish) == 0 {
			return err
		}

		cachePath, packageDependencies, err := getDependecnies(dependenciesToPublish)
		if err != nil {
			if tidyEnum.GetTidyValue() == tidyEnum.GetNoTidy() {
				return err
			}
			log.Error("Received an error:", err)
		}
		err = populateAndPublish(targetRepo, cachePath, tidyEnum, packageDependencies, &cache, serviceManager)
		if err != nil {
			if tidyEnum.GetTidyValue() == tidyEnum.GetNoTidy() {
				return err
			}
			log.Error("Received an error:", err)
		}

		log.Info(fmt.Sprintf("Done building and publishing %d go dependencies to Artifactory out of a total of %d dependencies.", cache.GetSuccesses(), cache.GetTotal()))
	}
	// Lets run the same command again now that all the dependencies were downloaded.
	// Need to run only if the command is not go mod download and go mod tidy since this was run by the CLI to download and publish to Artifactory
	if (!strings.Contains(goArg, "mod download") && !strings.Contains(goArg, "mod tidy")) || tidyEnum.GetTidyValue() == "" {
		if tidyEnum.GetTidyValue() != tidyEnum.GetNoTidy() {
			// Remove the go.sum file, since it includes information which is not up to date (it was created by the "go mod tidy" command executed without Artifactory
			if tidyEnum.GetTidyValue() != "" {
				err = removeGoSumFile(wd, rootProjectDir)
				if err != nil {
					log.Error("Received an error:", err)
				}
			}
		}
		return golang.RunGo(goArg)
	}
	return nil
}

// Download the dependencies from VCS and publish them to Artifactory.
func getDependecnies(dependenciesToPublish map[string]bool) (cachePath string, packageDependencies []dependencies.Package, err error) {
	cachePath, err = dependencies.GetCachePath()
	if err != nil {
		return
	}
	packageDependencies, err = dependencies.GetDependencies(cachePath, dependenciesToPublish)
	return
}

func populateAndPublish(targetRepo, cachePath string, tidyEnum golang.TidyEnum, packageDependencies []dependencies.Package, cache *golang.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	var dependenciesInterface dependencies.GoPackage
	if tidyEnum.GetTidyValue() != tidyEnum.GetNoTidy() {
		err := fileutils.CreateTempDirPath()
		if err != nil {
			return err
		}
		defer fileutils.RemoveTempDir()

		dependenciesInterface = &dependencies.PackageWithDeps{}
		err = dependenciesInterface.Init()
		if err != nil {
			return err
		}
	} else {
		dependenciesInterface = &dependencies.Package{}
	}

	cache.IncrementTotal(len(packageDependencies))
	for _, dep := range packageDependencies {
		dependenciesInterface = dependenciesInterface.New(cachePath, dep)
		err := dependenciesInterface.PopulateModAndPublish(targetRepo, cache, serviceManager)
		if err != nil {
			if tidyEnum.GetTidyValue() != tidyEnum.GetNoTidy() {
				log.Warn(err)
				continue
			}
			return err
		}
	}
	return nil
}

func removeGoSumFile(wd, rootDir string) error {
	log.Debug("Changing back to the working directory")
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
