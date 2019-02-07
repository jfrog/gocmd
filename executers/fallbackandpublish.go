package executers

import (
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

// Runs Go, with multiple fallbacks if needed and publish missing dependencies to Artifactory
func RunWithFallbacksAndPublish(goArg []string, targetRepo string, noRegistry bool, serviceManager *artifactory.ArtifactoryServicesManager) error {
	if !noRegistry {
		artDetails := serviceManager.GetConfig().GetArtDetails()
		err := setGoProxyWithApi(targetRepo, artDetails)
		if err != nil {
			return err
		}
	}

	err := cmd.RunGo(goArg)

	if err != nil {
		if dependencyNotFoundInArtifactory(err, noRegistry) {
			log.Info("Received", err.Error(), "from Artifactory. Trying to download dependencies from VCS...")
			err := os.Unsetenv(GOPROXY)
			if err != nil {
				return err
			}

			err = collectDependenciesAndPublish(targetRepo, true,  &Package{}, serviceManager)
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
