package executers

import (
	"os"

	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/gocmd/params"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Runs Go, with multiple fallbacks if needed and publish missing dependencies to Artifactory
func RunWithFallbacksAndPublish(goArg []string, resolverDeployer *params.ResolverDeployer) error {
	err := cmd.RunGo(goArg)

	if err != nil {
		if utils.DependencyNotFoundInArtifactory(err) {
			log.Info("Received", err.Error(), "from Artifactory. Trying to download dependencies from VCS...")
			err := os.Unsetenv(utils.GOPROXY)
			if err != nil {
				return err
			}

			err = collectDependenciesAndPublish(true, &Package{}, resolverDeployer)
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
