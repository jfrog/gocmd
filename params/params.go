package params

import (
	"github.com/jfrog/jfrog-client-go/artifactory"
	"reflect"
)

type ResolverDeployer struct {
	resolver *Params
	deployer *Params
}

func (resolverDeployer *ResolverDeployer) Resolver() *Params {
	return resolverDeployer.resolver
}

func (resolverDeployer *ResolverDeployer) SetResolver(resolver *Params) *ResolverDeployer {
	resolverDeployer.resolver = resolver
	return resolverDeployer
}

func (resolverDeployer *ResolverDeployer) Deployer() *Params {
	return resolverDeployer.deployer
}

func (resolverDeployer *ResolverDeployer) SetDeployer(deployer *Params) *ResolverDeployer {
	resolverDeployer.deployer = deployer
	return resolverDeployer
}

type Params struct {
	repo           string
	serviceManager *artifactory.ArtifactoryServicesManager
}

func (params *Params) Repo() string {
	return params.repo
}

func (params *Params) SetRepo(repo string) *Params {
	params.repo = repo
	return params
}

func (params *Params) ServiceManager() *artifactory.ArtifactoryServicesManager {
	return params.serviceManager
}

func (params *Params) SetServiceManager(serviceManager *artifactory.ArtifactoryServicesManager) *Params {
	params.serviceManager = serviceManager
	return params
}

// Returns true if goParams is empty
func (params *Params) IsEmpty() bool {
	return reflect.DeepEqual(*params, Params{})
}
