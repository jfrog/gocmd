package gocmd

import (
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/jfrog-client-go/auth"
)

func Run(goArg []string, server auth.ServiceDetails, repo string, noFallback bool) error {
	return cmd.RunGo(goArg, server, repo, noFallback)
}
