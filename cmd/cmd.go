package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jfrog/gocmd/executers/utils"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Used for masking basic auth credentials as part of a URL.
var protocolRegExp *gofrogcmd.CmdOutputPattern

func NewCmd() (*Cmd, error) {
	execPath, err := exec.LookPath("go")
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &Cmd{Go: execPath}, nil
}

func (config *Cmd) GetCmd() (cmd *exec.Cmd) {
	var cmdStr []string
	cmdStr = append(cmdStr, config.Go)
	cmdStr = append(cmdStr, config.Command...)
	cmdStr = append(cmdStr, config.CommandFlags...)
	cmd = exec.Command(cmdStr[0], cmdStr[1:]...)
	cmd.Dir = config.Dir
	return
}

func (config *Cmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *Cmd) GetStdWriter() io.WriteCloser {
	return config.StrWriter
}

func (config *Cmd) GetErrWriter() io.WriteCloser {
	return config.ErrWriter
}

type Cmd struct {
	Go           string
	Command      []string
	CommandFlags []string
	Dir          string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}

func GetGoVersion() (string, error) {
	goCmd, err := NewCmd()
	if err != nil {
		return "", err
	}
	goCmd.Command = []string{"version"}
	output, err := gofrogcmd.RunCmdOutput(goCmd)
	return output, errorutils.CheckError(err)
}

func RunGo(goArg []string, server auth.ServiceDetails, repo string, noFallback bool) error {
	utils.SetGoProxyWithApi(repo, server, noFallback)

	goCmd, err := NewCmd()
	if err != nil {
		return err
	}
	goCmd.Command = goArg
	err = prepareRegExp()
	if err != nil {
		return err
	}

	performPasswordMask, err := shouldMaskPassword()
	if err != nil {
		return err
	}
	if performPasswordMask {
		_, _, _, err = gofrogcmd.RunCmdWithOutputParser(goCmd, true, protocolRegExp)
	} else {
		_, _, _, err = gofrogcmd.RunCmdWithOutputParser(goCmd, true)
	}
	return errorutils.CheckError(err)
}

// GetGOPATH returns the location of the GOPATH
func getGOPATH() (string, error) {
	goCmd, err := NewCmd()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	goCmd.Command = []string{"env", "GOPATH"}
	output, err := gofrogcmd.RunCmdOutput(goCmd)
	if errorutils.CheckError(err) != nil {
		return "", fmt.Errorf("Could not find GOPATH env: %s", err.Error())
	}
	return strings.TrimSpace(parseGoPath(string(output))), nil
}

// Using go mod download {dependency} command to download the dependency
func DownloadDependency(dependencyName string) error {
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}
	log.Debug("Running go mod download -json", dependencyName)
	goCmd.Command = []string{"mod", "download", "-json", dependencyName}
	return errorutils.CheckError(gofrogcmd.RunCmd(goCmd))
}

// Runs 'go list -m' command and returns module name
func GetModuleNameByDir(projectDir string) (string, error) {
	cmdArgs, err := getListCmdArgs()
	if err != nil {
		return "", err
	}
	cmdArgs = append(cmdArgs, "-m")
	output, err := runDependenciesCmd(projectDir, cmdArgs)
	if err != nil {
		return "", err
	}
	lineOutput := strings.Split(output, "\n")
	return lineOutput[0], errorutils.CheckError(err)
}

// Gets go list command args according to go version
func getListCmdArgs() (cmdArgs []string, err error) {
	isAutoModify, err := automaticallyModifyMod()
	if err != nil {
		return []string{}, err
	}
	// Since version go1.16 build commands (like go build and go list) no longer modify go.mod and go.sum by default.
	if isAutoModify {
		return []string{"list"}, nil
	}
	return []string{"list", "-mod=mod"}, nil
}

// Runs go list -f {{with .Module}}{{.Path}}:{{.Version}}{{end}} all command and returns map of the dependencies
func GetDependenciesList(projectDir string) (map[string]bool, error) {
	cmdArgs, err := getListCmdArgs()
	if err != nil {
		return nil, err
	}
	output, err := runDependenciesCmd(projectDir, append(cmdArgs, "-f", "{{with .Module}}{{.Path}}@{{.Version}}{{end}}", "all"))
	if err != nil {
		return nil, err
	}
	return listToMap(output), errorutils.CheckError(err)
}

// Runs 'go mod graph' command and returns map that maps dependencies to their child dependencies slice
func GetDependenciesGraph(projectDir string) (map[string][]string, error) {
	output, err := runDependenciesCmd(projectDir, []string{"mod", "graph"})
	if err != nil {
		return nil, err
	}
	return graphToMap(output), errorutils.CheckError(err)
}

// Common function to run dependencies command for list or graph commands
func runDependenciesCmd(projectDir string, commandArgs []string) (string, error) {
	log.Info(fmt.Sprintf("Running 'go %s' in %s", strings.Join(commandArgs, " "), projectDir))
	var err error
	if projectDir == "" {
		projectDir, err = GetProjectRoot()
		if err != nil {
			return "", err
		}
	}
	// Read and store the details of the go.mod and go.sum files,
	// because they may change by the 'go mod graph' or 'go list' commands.
	modFileContent, modFileStat, err := GetFileDetails(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		log.Info("Dependencies were not collected for this build, since go.mod could not be found in", projectDir)
		return "", nil
	}
	sumFileContent, sumFileStat, err := GetGoSum(projectDir)
	if len(sumFileContent) > 0 && sumFileStat != nil {
		defer RestoreSumFile(projectDir, sumFileContent, sumFileStat)
	}
	goCmd, err := NewCmd()
	if err != nil {
		return "", err
	}
	goCmd.Command = commandArgs
	goCmd.Dir = projectDir

	err = prepareGlobalRegExp()
	if err != nil {
		return "", err
	}
	performPasswordMask, err := shouldMaskPassword()
	if err != nil {
		return "", err
	}
	var output string
	var executionError error
	if performPasswordMask {
		output, _, _, executionError = gofrogcmd.RunCmdWithOutputParser(goCmd, true, protocolRegExp)
	} else {
		output, _, _, executionError = gofrogcmd.RunCmdWithOutputParser(goCmd, true)
	}
	if len(output) != 0 {
		log.Debug(output)
	}
	if executionError != nil {
		// If the command fails, the mod stays the same, therefore, don't need to be restored.
		errorString := fmt.Sprintf("Failed running Go command: 'go %s' in %s with error: '%s'", strings.Join(commandArgs, " "), projectDir, executionError.Error())
		return "", errorutils.CheckError(errors.New(errorString))
	}

	// Restore the the go.mod and go.sum files, to make sure they stay the same as before
	// running the "go mod graph" command.
	err = ioutil.WriteFile(filepath.Join(projectDir, "go.mod"), modFileContent, modFileStat.Mode())
	if err != nil {
		return "", err
	}
	return output, err
}

// Returns the root dir where the go.mod located.
func GetProjectRoot() (string, error) {
	// Create a map to store all paths visited, to avoid running in circles.
	visitedPaths := make(map[string]bool)
	// Get the current directory.
	wd, err := os.Getwd()
	if err != nil {
		return wd, errorutils.CheckError(err)
	}
	defer os.Chdir(wd)

	// Get the OS root.
	osRoot := os.Getenv("SYSTEMDRIVE")
	if osRoot != "" {
		// If this is a Windows machine:
		osRoot += "\\"
	} else {
		// Unix:
		osRoot = "/"
	}

	// Check if the current directory includes the go.mod file. If not, check the parent directpry
	// and so on.
	for {
		// If the go.mod is found the current directory, return the path.
		exists, err := fileutils.IsFileExists(filepath.Join(wd, "go.mod"), false)
		if err != nil || exists {
			return wd, err
		}

		// If this the OS root, we can stop.
		if wd == osRoot {
			break
		}

		// Save this path.
		visitedPaths[wd] = true
		// CD to the parent directory.
		wd = filepath.Dir(wd)
		os.Chdir(wd)

		// If we already visited this directory, it means that there's a loop and we can stop.
		if visitedPaths[wd] {
			return "", errorutils.CheckError(errors.New("Could not find go.mod for project."))
		}
	}

	return "", errorutils.CheckError(errors.New("Could not find go.mod for project."))
}

// GetGoModCachePath returns the location of the go module cache
func GetGoModCachePath() (string, error) {
	goPath, err := getGOPATH()
	if err != nil {
		return "", err
	}
	return filepath.Join(goPath, "pkg", "mod"), nil
}

// GetCachePath returns the location of downloads dir insied the GOMODCACHE
func GetCachePath() (string, error) {
	goModCachePath, err := GetGoModCachePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(goModCachePath, "cache", "download"), nil
}

func parseGoPath(goPath string) string {
	if runtime.GOOS == "windows" {
		goPathSlice := strings.Split(goPath, ";")
		return goPathSlice[0]
	}
	goPathSlice := strings.Split(goPath, ":")
	return goPathSlice[0]
}
