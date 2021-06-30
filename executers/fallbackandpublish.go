package executers

import (
	"github.com/jfrog/gocmd/cmd"
)

// Runs Go, with multiple fallbacks if needed and publish missing dependencies to Artifactory
func RunWithFallbacks(goArg []string) error {
	return cmd.RunGo(goArg)
}
