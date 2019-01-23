package executers

import (
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func ExecuteGoCentral(goArg string) error {
	register(&goCentralExecutor{goArg: goArg})
	serviceManager, err := createGoCentralServiceManager()
	if err != nil {
		return err
	}
	return ExecuteGo(goArg, "gocenter-virtual", false, serviceManager)
}

type goCentralExecutor struct{
	goArg string
}

func (gce *goCentralExecutor) execute() error {
	return cmd.RunGo(gce.goArg)
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
