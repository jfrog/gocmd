package executers

import (
	"github.com/jfrog/gocmd/utils/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
	"os"
)

func Execute(goArg, url string) error {
	register(&resolverExecuter{goArg: goArg})
	serviceManager, err := createGoCentralServiceManager(url)
	if err != nil {
		return err
	}
	return ExecuteGo(goArg, false, serviceManager)
}

type resolverExecuter struct {
	goArg string
}

// Run Go without GOPROXY
func (re *resolverExecuter) execute() error {
	return cmd.RunGo(re.goArg)
}

func (re *resolverExecuter) SetGoProxyEnvVar(details auth.ArtifactoryDetails) error {
	rtUrl, err := url.Parse(details.GetUrl())
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = os.Setenv(cmd.GOPROXY, rtUrl.String())
	return errorutils.CheckError(err)
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
