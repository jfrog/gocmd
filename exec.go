package gocmd

import (
	"github.com/jfrog/gocmd/executers"
	"github.com/jfrog/gocmd/params"
)

func RunWithFallbacksAndPublish(goArg []string, resolverDeployer *params.ResolverDeployer) error {
	return executers.RunWithFallbacksAndPublish(goArg, resolverDeployer)
}

func RunWithFallback(goArg []string, url string) error {
	return executers.RunWithFallback(goArg, url)
}
