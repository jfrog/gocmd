package executers

import (
	"net/url"
	"os"

	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	artifactoryAuth "github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/auth"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Run Go with fallback to VCS without publish
func RunWithFallback(goArg []string, url string) error {
	serviceManager, err := createGoCentralServiceManager(url)
	if err != nil {
		return err
	}

	artDetails := serviceManager.GetConfig().GetServiceDetails()
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

func setGoProxyWithoutApi(details auth.ServiceDetails) error {
	rtUrl, err := url.Parse(details.GetUrl())
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = os.Setenv(utils.GOPROXY, rtUrl.String())
	return errorutils.CheckError(err)
}

func createGoCentralServiceManager(url string) (artifactory.ArtifactoryServicesManager, error) {
	artifactoryDetails := artifactoryAuth.NewArtifactoryDetails()
	artifactoryDetails.SetUrl(clientutils.AddTrailingSlashIfNeeded(url))
	serviceConfig, err := clientConfig.NewConfigBuilder().SetServiceDetails(artifactoryDetails).SetDryRun(false).Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}
