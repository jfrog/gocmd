package tests

import (
	"github.com/jfrog/jfrog-client-go/utils/tests"
	"testing"
)

var gocmdTestDir = "github.com/jfrog/gocmd/tests"

func TestUnitTests(t *testing.T) {
	packages := tests.GetTestPackages("./../...")
	packages = tests.ExcludeTestsPackage(packages, gocmdTestDir)
	tests.RunTests(packages, false)
}
