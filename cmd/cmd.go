package cmd

import (
	"errors"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// Used for masking basic auth credentials as part of a URL.
var protocolRegExp *gofrogcmd.CmdOutputPattern

// Used for identifying an "unrecognized import" log line when executing the go client.
var unrecognizedImportRegExp *gofrogcmd.CmdOutputPattern

// Used for identifying an "404 not found" log line when executing the go client.
// Compatible with the log message format before go 1.13.
var notFoundRegExp *gofrogcmd.CmdOutputPattern

// Used for identifying an "404 not found" log line when executing the go client.
// Compatible with the log message format starting from go 1.13.
var notFoundGo113RegExp *gofrogcmd.CmdOutputPattern

// Used for identifying an "unknown revision" log line when executing the go client.
var unknownRevisionRegExp *gofrogcmd.CmdOutputPattern

// Used for identifying a case where the zip is not found in the repository,
// using the go client log output,
var notFoundZipRegExp *gofrogcmd.CmdOutputPattern

func NewCmd() (*Cmd, error) {
	execPath, err := exec.LookPath("go")
	if err != nil {
		return nil, errorutils.WrapError(err)
	}
	return &Cmd{Go: execPath}, nil
}

func (config *Cmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, config.Go)
	cmd = append(cmd, config.Command...)
	cmd = append(cmd, config.CommandFlags...)
	return exec.Command(cmd[0], cmd[1:]...)
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
	return output, errorutils.WrapError(err)
}

func RunGo(goArg []string) error {
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}
	goCmd.Command = goArg
	err = prepareRegExp()
	if err != nil {
		return err
	}

	_, _, _, err = gofrogcmd.RunCmdWithOutputParser(goCmd, true, protocolRegExp, notFoundRegExp, notFoundGo113RegExp, unrecognizedImportRegExp, unknownRevisionRegExp, notFoundZipRegExp)
	return errorutils.WrapError(err)
}

// Using go mod download {dependency} command to download the dependency
func DownloadDependency(dependencyName string) error {
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}
	log.Debug("Running go mod download -json", dependencyName)
	goCmd.Command = []string{"mod", "download", "-json", dependencyName}
	return errorutils.WrapError(gofrogcmd.RunCmd(goCmd))
}

// Runs go mod graph command and returns slice of the dependencies
func GetDependenciesGraph() (map[string]bool, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	projectDir, err := GetProjectRoot()
	if err != nil {
		return nil, err
	}

	// Read and store the details of the go.mod and go.sum files,
	// because they may change by the "go mod graph" command.
	modFileContent, modFileStat, err := GetFileDetails(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return nil, err
	}
	sumFileContent, sumFileStat, err := GetGoSum(projectDir)
	if len(sumFileContent) > 0 && sumFileStat != nil {
		defer RestoreSumFile(projectDir, sumFileContent, sumFileStat)
	}

	log.Info("Running 'go mod graph' in", pwd)
	goCmd, err := NewCmd()
	if err != nil {
		return nil, err
	}
	goCmd.Command = []string{"mod", "graph"}

	err = prepareGlobalRegExp()
	if err != nil {
		return nil, err
	}
	output, _, _, err := gofrogcmd.RunCmdWithOutputParser(goCmd, true, protocolRegExp, notFoundRegExp, unrecognizedImportRegExp, unknownRevisionRegExp)
	if len(output) != 0 {
		log.Debug(output)
	}

	if err != nil {
		// If the command fails, the mod stays the same, therefore, don't need to be restored.
		return nil, errorutils.WrapError(err)
	}

	// Restore the the go.mod and go.sum files, to make sure they stay the same as before
	// running the "go mod graph" command.
	err = ioutil.WriteFile(filepath.Join(projectDir, "go.mod"), modFileContent, modFileStat.Mode())
	if err != nil {
		return nil, err
	}

	return outputToMap(output), errorutils.WrapError(err)
}

// Using go mod download command to download all the dependencies before publishing to Artifactory
func RunGoModTidy() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	log.Info("Running 'go mod tidy' in", pwd)
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}

	goCmd.Command = []string{"mod", "tidy"}
	_, err = gofrogcmd.RunCmdOutput(goCmd)
	return err
}

func RunGoModInit(moduleName string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	log.Info("Running 'go mod init' in", pwd)
	goCmd, err := NewCmd()
	if err != nil {
		return err
	}

	goCmd.Command = []string{"mod", "init", moduleName}
	_, _, _, err = gofrogcmd.RunCmdWithOutputParser(goCmd, true)
	return err
}

// Returns the root dir where the go.mod located.
func GetProjectRoot() (string, error) {
	// Create a map to store all paths visited, to avoid running in circles.
	visitedPaths := make(map[string]bool)
	// Get the current directory.
	wd, err := os.Getwd()
	if err != nil {
		return wd, errorutils.WrapError(err)
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
			return "", errorutils.WrapError(errors.New("Could not find go.mod for project."))
		}
	}

	return "", errorutils.WrapError(errors.New("Could not find go.mod for project."))
}
