package executers

import (
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

// Runs Go, with multiple fallbacks if needed and publish missing dependencies to Artifactory
func RunWithFallbacksAndPublish(goArg []string, targetRepo string, noRegistry, publishDeps bool, serviceManager *artifactory.ArtifactoryServicesManager) error {
	if !noRegistry {
		artDetails := serviceManager.GetConfig().GetArtDetails()
		err := utils.SetGoProxyWithApi(targetRepo, artDetails)
		if err != nil {
			return err
		}
	}

	err := cmd.RunGo(goArg)

	if err != nil {
		if utils.DependencyNotFoundInArtifactory(err, noRegistry) {
			log.Info("Received", err.Error(), "from Artifactory. Trying to download dependencies from VCS...")
			err := os.Unsetenv(utils.GOPROXY)
			if err != nil {
				return err
			}

			err = collectDependenciesAndPublish(targetRepo, true, publishDeps, &Package{}, serviceManager)
			if err != nil {
				return err
			}
			return cmd.RunGo(goArg)
		} else {
			return err
		}
	}
	return nil
}
