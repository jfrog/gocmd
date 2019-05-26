package gocmd

import (
	"github.com/jfrog/gocmd/executers"
	"github.com/jfrog/gocmd/params"
)

func RecursivePublish(goModEditMessage string, resolverDeployer *params.ResolverDeployer) error {
	return executers.RecursivePublish(goModEditMessage, resolverDeployer)
}

func RunWithFallbacksAndPublish(goArg []string, noRegistry, publishDeps bool, resolverDeployer *params.ResolverDeployer) error {
	return executers.RunWithFallbacksAndPublish(goArg, noRegistry, publishDeps, resolverDeployer)
}

func RunWithFallback(goArg []string, url string) error {
	return executers.RunWithFallback(goArg, url)
}
