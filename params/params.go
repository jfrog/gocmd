package params

import (
	"github.com/jfrog/jfrog-client-go/artifactory"
	"reflect"
)

type GoResolverDeployer struct {
	resolver *GoParams
	deployer *GoParams
}

func (resolverDeployer *GoResolverDeployer) Resolver() *GoParams {
	return resolverDeployer.resolver
}

func (resolverDeployer *GoResolverDeployer) SetResolver(resolver *GoParams) *GoResolverDeployer {
	resolverDeployer.resolver = resolver
	return resolverDeployer
}

func (resolverDeployer *GoResolverDeployer) Deployer() *GoParams {
	return resolverDeployer.deployer
}

func (resolverDeployer *GoResolverDeployer) SetDeployer(deployer *GoParams) *GoResolverDeployer {
	resolverDeployer.deployer = deployer
	return resolverDeployer
}

type GoParams struct {
	repo           string
	serviceManager *artifactory.ArtifactoryServicesManager
}

func (goParams *GoParams) Repo() string {
	return goParams.repo
}

func (goParams *GoParams) SetRepo(repo string) *GoParams {
	goParams.repo = repo
	return goParams
}

func (goParams *GoParams) ServiceManager() *artifactory.ArtifactoryServicesManager {
	return goParams.serviceManager
}

func (goParams *GoParams) SetServiceManager(serviceManager *artifactory.ArtifactoryServicesManager) *GoParams {
	goParams.serviceManager = serviceManager
	return goParams
}

// Returns true if goParams is empty
func (goParams *GoParams) IsEmpty() bool {
	return reflect.DeepEqual(*goParams, GoParams{})
}
