package executers

import (
	"fmt"
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/gocmd/params"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

// Runs Go, with multiple fallbacks if needed and publish missing dependencies to Artifactory
func RunWithFallbacksAndPublish(goArg []string, noRegistry, publishDeps bool, resolverDeployer *params.ResolverDeployer) error {
	if !noRegistry {
		resolver := resolverDeployer.Resolver()
		if resolver == nil || resolver.IsEmpty() {
			return errorutils.CheckError(fmt.Errorf("Missing resolver information"))
		}
		artDetails := resolver.ServiceManager().GetConfig().GetArtDetails()
		err := utils.SetGoProxyWithApi(resolverDeployer.Resolver().Repo(), artDetails)
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

			err = collectDependenciesAndPublish(true, publishDeps, &Package{}, resolverDeployer)
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
