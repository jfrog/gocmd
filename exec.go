package gocmd

import (
	"github.com/jfrog/gocmd/executers"
)

func RunWithFallback(goArg []string, url string) error {
	return executers.RunWithFallbacks(goArg)
}
