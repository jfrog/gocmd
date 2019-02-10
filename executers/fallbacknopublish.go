package executers

import (
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
	"os"
)

// Run Go with fallback to VCS without publish
func RunWithFallback(goArg []string, url string) error {
	serviceManager, err := createGoCentralServiceManager(url)
	if err != nil {
		return err
	}

	artDetails := serviceManager.GetConfig().GetArtDetails()
	err = setGoProxyWithoutApi(artDetails)
	if err != nil {
		return err
	}
	err = cmd.RunGo(goArg)

	if err != nil {
		log.Info("Received", err.Error(), "from proxy. Trying to download dependencies from VCS...")
		err := os.Unsetenv(utils.GOPROXY)
		if err != nil {
			return err
		}
		return cmd.RunGo(goArg)
	}
	return nil
}

func setGoProxyWithoutApi(details auth.ArtifactoryDetails) error {
	rtUrl, err := url.Parse(details.GetUrl())
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = os.Setenv(utils.GOPROXY, rtUrl.String())
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
