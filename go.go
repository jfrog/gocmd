package gocmd

import (
	"github.com/jfrog/gocmd/golang"
	"github.com/jfrog/gocmd/golang/project"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

func ExecuteGoCentral(goArg string) error {
	serviceManager, err := createGoCentralServiceManager()
	if err != nil {
		return err
	}
	return ExecuteGo(goArg, "gocenter-virtual", golang.TidyEnum{}, false, serviceManager)
}

func ExecuteGo(goArg, targetRepo string, tidyEnum golang.TidyEnum, noRegistry bool, serviceManager *artifactory.ArtifactoryServicesManager) error {
	if !noRegistry {
		artDetails := serviceManager.GetConfig().GetArtDetails()
		err := golang.SetGoProxyEnvVar(artDetails.GetUrl(), artDetails.GetUser(), artDetails.GetPassword(), targetRepo)
		if err != nil {
			return err
		}
	}

	err := golang.RunGo(goArg)

	if err != nil {
		if dependencyNotFoundInArtifactory(err, noRegistry) {
			log.Info("Received", err.Error(), "from Artifactory. Trying download the dependencies from the VCS...")
			err = project.DownloadFromVcsAndPublish(targetRepo, goArg, tidyEnum, serviceManager)
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

func createGoCentralServiceManager() (*artifactory.ArtifactoryServicesManager, error) {
	artifactoryDetails := auth.NewArtifactoryDetails()
	artifactoryDetails.SetUrl("https://gocenter.jfrog.info/gocenter/")
	serviceConfig, err := artifactory.NewConfigBuilder().SetArtDetails(artifactoryDetails).SetDryRun(false).SetLogger(log.Logger).Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}