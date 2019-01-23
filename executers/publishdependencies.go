package executers

import (
	"github.com/jfrog/gocmd/dependencies"
	"github.com/jfrog/gocmd/utils/cache"
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
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
	dependenciesInterface := &dependencies.Package{}

	pd := &publishDependencies{wd: wd, dependenciesInterface: dependenciesInterface, targetRepo: targetRepo, serviceManager: serviceManager, goArg:goArg}
	register(pd)

	return ExecuteGo(goArg, targetRepo, noRegistry, serviceManager)
}

type publishDependencies struct {
	wd                    string
	targetRepo            string
	goArg                 string
	serviceManager        *artifactory.ArtifactoryServicesManager
	dependenciesInterface dependencies.GoPackage
}

func (pd *publishDependencies) execute() error {
	rootProjectDir, err := cmd.GetProjectRoot()
	if err != nil {
		return err
	}
	cache := cache.DependenciesCache{}
	dependenciesToPublish, err := collectDependencies(pd.targetRepo, rootProjectDir, &cache, pd.serviceManager)
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
