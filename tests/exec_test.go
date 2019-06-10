package tests

import (
	"github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"testing"
)

var gocmdTestDir = "github.com/jfrog/gocmd/tests"

func TestUnitTests(t *testing.T) {
	log.SetLogger(log.NewLogger(log.DEBUG, nil))
	packages := tests.GetTestPackages("./../...")
	packages = tests.ExcludeTestsPackage(packages, gocmdTestDir)
	tests.RunTests(packages, false)
}
