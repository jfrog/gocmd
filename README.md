# gocmd

|Branch|Status|
|:---:|:---:|
|master|[![Build status](https://ci.appveyor.com/api/projects/status/a5wv9lp4eg1v99a3/branch/master?svg=true)](https://ci.appveyor.com/project/jfrog-ecosystem/gocmd/branch/master)

## General

*gocmd* is a library which provides Go APIs to performs actions on JFrog Artifactory from your Go application related to working with Go packages.
The project is still relatively new, and its APIs may therefore change frequently between releases.
The library can be used as a go-module, which should be added to your project's go.mod file. As a reference you may look at [JFrog CLI](https://github.com/jfrog/jfrog-cli-go)'s [go.mod](https://github.com/jfrog/jfrog-cli-go/blob/master/go.mod) file, which uses this library as a dependency.

## General APIs

## Tests
To run tests on the source code, use the following command:
````
go test -v github.com/jfrog/gocmd/tests -timeout 0
````
