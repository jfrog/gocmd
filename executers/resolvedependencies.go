package executers

import (
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func Execute(goArg, url, repo string) error {
	register(&resolverExecuter{goArg: goArg})
	serviceManager, err := createGoCentralServiceManager(url)
	if err != nil {
		return err
	}
	return ExecuteGo(goArg, repo, false, serviceManager)
}

type resolverExecuter struct{
	goArg string
}

// Run Go without GOPROXY
func (re *resolverExecuter) execute() error {
	return cmd.RunGo(re.goArg)
}

func createGoCentralServiceManager(url string) (*artifactory.ArtifactoryServicesManager, error) {
	artifactoryDetails := auth.NewArtifactoryDetails()
	artifactoryDetails.SetUrl(clientutils.AddTrailingSlashIfNeeded(url))
	serviceConfig, err := artifactory.NewConfigBuilder().SetArtDetails(artifactoryDetails).SetDryRun(false).SetLogger(log.Logger).Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(&artifactoryDetails, serviceConfig)
}
