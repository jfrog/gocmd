package executers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"

	"github.com/jfrog/gocmd/cache"
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/gocmd/params"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	multifilereader "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/errors"
)

const (
	FailedToRetrieve          = "Failed to retrieve"
	FromBothArtifactoryAndVcs = "from both Artifactory and VCS"
)

// Resolve artifacts from VCS and publish the missing artifacts to Artifactory
func collectDependenciesAndPublish(failOnError, publishDeps bool, dependenciesInterface GoPackage, resolverDeployer *params.ResolverDeployer) error {
	rootProjectDir, err := cmd.GetProjectRoot()
	if err != nil {
		return err
	}
	cache := cache.DependenciesCache{}
	// The collection of project dependencies requires the resolver information
	dependenciesToPublish, err := collectProjectDependencies(resolverDeployer.Resolver().Repo(), rootProjectDir, &cache, resolverDeployer.Resolver().ServiceManager().GetConfig().GetServiceDetails())
	if err != nil || len(dependenciesToPublish) == 0 {
		return err
	}
	cachePath, packageDependencies, err := getDependencies(dependenciesToPublish)
	if err != nil {
		if failOnError {
			return err
		}
		log.Error("Received an error retrieving project dependencies:", err)
	}

	// If need to publish the missing depedencies to Artifactory
	if publishDeps {
		// If we need publish the missing depedencies to an Artifactory server or repo, which are
		// different then the resolution repo, then we have no information about which depedencies are actually
		// missing and therefore should be published.
		// Let's find out which depedencies are missing.
		if !reflect.DeepEqual(resolverDeployer.Resolver(), resolverDeployer.Deployer()) {
			err = findMissingDepedencies(&cache, dependenciesToPublish, resolverDeployer)
			if err != nil {
				return err
			}
		}
		// Publish the missing dependencies to Artifactory.
		err = populateAndPublish(resolverDeployer.Deployer().Repo(), cachePath, dependenciesInterface, packageDependencies, &cache, resolverDeployer.Deployer().ServiceManager())
		if err != nil {
			return err
		}
	}

	utils.LogFinishedMsg(&cache)
	return nil
}

// Performs a head request to the deployer server to select the dependencies that need to be published to that server
func findMissingDepedencies(cache *cache.DependenciesCache, dependenciesToPublish map[string]bool, resolverDeployer *params.ResolverDeployer) error {
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return err
	}
	cacheDependenciesMap := cache.GetMap()
	for module := range dependenciesToPublish {
		nameAndVersion := strings.Split(module, "@")
		// Perform a head request for the deployer server
		resp, err := performHeadRequest(resolverDeployer.Deployer().ServiceManager().GetConfig().GetServiceDetails(), client, resolverDeployer.Deployer().Repo(), nameAndVersion[0], nameAndVersion[1])
		if err != nil {
			return err
		}
		// Change the cache map to indicate which dependencies are missing in the deployer server.
		var dependencyExists bool
		if resp.StatusCode == 200 {
			dependencyExists = true
		} else if resp.StatusCode == 404 {
			dependencyExists = false
		} else {
			return errorutils.CheckError(fmt.Errorf("Artifactory response for %s:%d", module, resp.StatusCode))
		}
		cacheDependenciesMap[goModEncode(nameAndVersion[0])+":"+goModEncode(nameAndVersion[1])] = dependencyExists
	}
	return nil
}

func populateAndPublish(targetRepo, cachePath string, dependenciesInterface GoPackage, packageDependencies []Package, cache *cache.DependenciesCache, serviceManager artifactory.ArtifactoryServicesManager) error {
	cache.IncrementTotal(len(packageDependencies))
	for _, dep := range packageDependencies {
		dependenciesInterface = dependenciesInterface.New(cachePath, dep)
		err := dependenciesInterface.PopulateModAndPublish(targetRepo, cache, serviceManager)
		if err != nil {
			// If using recursive publish - the error always nil. If we got here, means that this error happened when not using recursive publish.
			return err
		}
	}
	return nil
}

// Collects the dependencies of the project
func collectProjectDependencies(targetRepo, rootProjectDir string, cache *cache.DependenciesCache, auth auth.ServiceDetails) (map[string]bool, error) {
	dependenciesMap, err := getDependenciesGraphWithFallback(targetRepo, auth)
	if err != nil {
		return nil, err
	}
	replaceDependencies, err := getReplaceDependencies()
	if err != nil {
		return nil, err
	}

	// Merge replaceDependencies with dependenciesToPublish
	mergeReplaceDependenciesWithGraphDependencies(replaceDependencies, dependenciesMap)
	sumFileContent, sumFileStat, err := cmd.GetGoSum(rootProjectDir)
	if err != nil {
		return nil, err
	}
	if len(sumFileContent) > 0 && sumFileStat != nil {
		defer cmd.RestoreSumFile(rootProjectDir, sumFileContent, sumFileStat)
	}
	projectDependencies, err := downloadDependencies(targetRepo, cache, dependenciesMap, auth)
	if err != nil {
		return projectDependencies, err
	}
	return projectDependencies, nil
}

func downloadDependencies(targetRepo string, cache *cache.DependenciesCache, depSlice map[string]bool, auth auth.ServiceDetails) (map[string]bool, error) {
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return nil, err
	}
	cacheDependenciesMap := cache.GetMap()
	dependenciesMap := map[string]bool{}
	for module := range depSlice {
		nameAndVersion := strings.Split(module, "@")
		resp, err := performHeadRequest(auth, client, targetRepo, nameAndVersion[0], nameAndVersion[1])
		if err != nil {
			return dependenciesMap, err
		}

		if resp.StatusCode == 200 {
			cacheDependenciesMap[goModEncode(nameAndVersion[0])+":"+goModEncode(nameAndVersion[1])] = true
			err = downloadDependency(true, module, targetRepo, auth)
			dependenciesMap[module] = true
		} else if resp.StatusCode == 404 {
			cacheDependenciesMap[goModEncode(nameAndVersion[0])+":"+goModEncode(nameAndVersion[1])] = false
			err = downloadDependency(false, module, targetRepo, nil)
			dependenciesMap[module] = false
		}

		if err != nil {
			return dependenciesMap, err
		}
	}
	return dependenciesMap, nil
}

func performHeadRequest(auth auth.ServiceDetails, client *httpclient.HttpClient, targetRepo, module, version string) (*http.Response, error) {
	url := auth.GetUrl() + "api/go/" + targetRepo + "/" + module + "/@v/" + version + ".mod"
	resp, _, err := client.SendHead(url, auth.CreateHttpClientDetails())
	if err != nil {
		return nil, err
	}
	log.Debug("Artifactory head request response for", url, ":", resp.StatusCode)
	return resp, nil
}

// Creating dependency with the mod file in the temp directory
func createDependencyInTemp(zipPath, tempDir string) (err error) {
	multiReader, err := multifilereader.NewMultiFileReaderAt([]string{zipPath})
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = fileutils.Unzip(multiReader, multiReader.Size(), tempDir)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

// Returns the actual path to the dependency.
// If in the path there are capital letters, the Go convention is to use "!" before the letter.
// The letter itself in lowercase.
func goModEncode(name string) string {
	path := ""
	for _, letter := range name {
		if unicode.IsUpper(letter) {
			path += "!" + strings.ToLower(string(letter))
		} else {
			path += string(letter)
		}
	}
	return path
}

// Returns the path to the dependency decoded form lower case to upper case
// If in the path there are capital letters, the Go convention is to use "!" before the letter.
// The letter itself in lowercase. This function will decode back to Upper case
func goModDecode(name string) string {
	var str string
	for i := 0; i < len(name); i++ {
		if string(name[i]) == "!" {
			if i < len(name)-1 {
				r := rune(name[i+1])
				str += string(unicode.ToUpper(r))
				i++
			}
		} else {
			str += string(name[i])
		}
	}
	return str
}

// Runs the go mod download command. Should set first the environment variable of GoProxy
func downloadDependency(downloadFromArtifactory bool, fullDependencyName, targetRepo string, auth auth.ServiceDetails) error {
	var err error
	if downloadFromArtifactory {
		log.Debug("Downloading dependency from Artifactory:", fullDependencyName)
		err = utils.SetGoProxyWithApi(targetRepo, auth)
	} else {
		log.Debug("Downloading dependency from VCS:", fullDependencyName)
		err = os.Unsetenv(utils.GOPROXY)
	}
	if errorutils.CheckError(err) != nil {
		return err
	}

	err = cmd.DownloadDependency(fullDependencyName)
	return err
}

// Downloads the mod file from Artifactory to the Go cache
func downloadModFileFromArtifactoryToLocalCache(cachePath, targetRepo, name, version string, auth auth.ServiceDetails, client *httpclient.HttpClient) string {
	pathToModuleCache := filepath.Join(cachePath, name, "@v")
	dirExists, err := fileutils.IsDirExists(pathToModuleCache, false)
	if err != nil {
		log.Error(fmt.Sprintf("Received an error: %s for %s@%s", err, name, version))
		return ""
	}

	if dirExists {
		url := auth.GetUrl() + "api/go/" + targetRepo + "/" + name + "/@v/" + version + ".mod"
		log.Debug("Downloading mod file from Artifactory:", url)
		downloadFileDetails := &httpclient.DownloadFileDetails{
			FileName: version + ".mod",
			// Artifactory URL
			DownloadPath:  url,
			LocalPath:     pathToModuleCache,
			LocalFileName: version + ".mod",
		}
		resp, err := client.DownloadFile(downloadFileDetails, "", auth.CreateHttpClientDetails(), 3, false)
		if err != nil {
			log.Error(fmt.Sprintf("Received an error %s downloading a file: %s to the local path: %s", err.Error(), downloadFileDetails.FileName, downloadFileDetails.LocalPath))
			return ""
		}

		log.Debug(fmt.Sprintf("Received %d from Artifactory %s", resp.StatusCode, url))
		return filepath.Join(downloadFileDetails.LocalPath, downloadFileDetails.LocalFileName)
	}
	return ""
}

func downloadAndCreateDependency(cachePath, name, version, fullDependencyName, targetRepo string, downloadedFromArtifactory bool, auth auth.ServiceDetails) (*Package, error) {
	// Dependency is missing within the cache. Need to download it...
	err := downloadDependency(downloadedFromArtifactory, fullDependencyName, targetRepo, auth)
	if err != nil {
		return nil, err
	}
	// Now that this dependency in the cache, get the dependency object
	dep, err := createDependency(cachePath, name, version)
	if err != nil {
		return nil, err
	}
	return dep, nil
}

func shouldDownloadFromArtifactory(module, version, targetRepo string, auth auth.ServiceDetails, client *httpclient.HttpClient) (bool, error) {
	res, err := performHeadRequest(auth, client, targetRepo, module, version)
	if err != nil {
		return false, err
	}
	if res.StatusCode == 200 {
		return true, nil
	}
	return false, nil
}

func GetDependencies(cachePath string, moduleSlice map[string]bool) ([]Package, error) {
	var deps []Package
	for module := range moduleSlice {
		moduleInfo := strings.Split(module, "@")
		name := goModEncode(moduleInfo[0])
		dep, err := createDependency(cachePath, name, goModEncode(moduleInfo[1]))
		if err != nil {
			return nil, err
		}
		if dep != nil {
			deps = append(deps, *dep)
		}
	}
	return deps, nil
}

// Creates a go dependency.
// Returns a nil value in case the dependency does not include a zip in the cache.
func createDependency(cachePath, dependencyName, version string) (*Package, error) {
	// We first check if the this dependency has a zip binary in the local go cache.
	// If it does not, nil is returned. This seems to be a bug in go.
	zipPath, err := getPackageZipLocation(cachePath, dependencyName, version)

	if err != nil {
		return nil, err
	}

	if zipPath == "" {
		return nil, nil
	}

	dep := Package{}

	dep.id = strings.Join([]string{dependencyName, version}, ":")
	dep.version = version
	dep.zipPath = zipPath
	dep.modPath = filepath.Join(cachePath, dependencyName, "@v", version+".mod")
	dep.infoPath = filepath.Join(cachePath, dependencyName, "@v", version+".info")
	dep.modContent, err = ioutil.ReadFile(dep.modPath)
	if err != nil {
		return &dep, errorutils.CheckError(err)
	}

	return &dep, nil
}

// Returns the path to the package zip file if exists.
func getPackageZipLocation(cachePath, dependencyName, version string) (string, error) {
	zipPath, err := getPackagePathIfExists(cachePath, dependencyName, version)
	if err != nil {
		return "", err
	}

	if zipPath != "" {
		return zipPath, nil
	}

	zipPath, err = getPackagePathIfExists(filepath.Dir(cachePath), dependencyName, version)

	if err != nil {
		return "", err
	}

	return zipPath, nil
}

// Validates if the package zip file exists.
func getPackagePathIfExists(cachePath, dependencyName, version string) (zipPath string, err error) {
	zipPath = filepath.Join(cachePath, dependencyName, "@v", version+".zip")
	fileExists, err := fileutils.IsFileExists(zipPath, false)
	if err != nil {
		log.Warn(fmt.Sprintf("Could not find zip binary for dependency '%s' at %s.", dependencyName, zipPath))
		return "", err
	}
	// Zip binary does not exist, so we skip it by returning a nil dependency.
	if !fileExists {
		log.Debug("The following file is missing:", zipPath)
		return "", nil
	}
	return zipPath, nil
}

func mergeReplaceDependenciesWithGraphDependencies(replaceDeps []string, graphDeps map[string]bool) {
	for _, replaceLine := range replaceDeps {
		// Remove unnecessary spaces
		replaceLine = strings.TrimSpace(replaceLine)
		log.Debug("Working on the following replace line:", replaceLine)
		// Split to get the right side that is the replace of the dependency
		replaceDeps := strings.Split(replaceLine, "=>")
		// Perform validation
		if len(replaceDeps) < 2 {
			log.Debug("The following replace line includes less then two elements", replaceDeps)
			continue
		}
		// Strip "replace" keyword if present and split module to replace to find name and version which is optional
		moduleToReplaceInfo := strings.Split(strings.TrimSpace(strings.Replace(replaceDeps[0], "replace ", "", 1)), " ")
		moduleNameToReplace := strings.TrimSpace(moduleToReplaceInfo[0])
		var moduleVersionToReplace string
		if len(moduleToReplaceInfo) > 1 {
			moduleVersionToReplace = strings.TrimSpace(moduleToReplaceInfo[1])
		}

		replacesInfo := strings.TrimSpace(replaceDeps[1])
		newDependency := strings.Split(replacesInfo, " ")
		if len(newDependency) != 2 {
			log.Debug("The replacer is not pointing to a VCS version", newDependency[0])
			// For local replacement, only removal from the map is required as no extra deps need to be added for download
			removeFromDepsGraph(graphDeps, moduleNameToReplace, moduleVersionToReplace)
			continue
		}

		// Remove the dependencies from graph matching module to replace to avoid downloading deps which will never
		// be used as they are replaced
		removedCount := removeFromDepsGraph(graphDeps, moduleNameToReplace, moduleVersionToReplace)
		// Only add the replacement dependency to the map if it is replacing at least one dependency
		if removedCount > 0 {
			graphDeps[newDependency[0]+"@"+newDependency[1]] = true
		}
	}
}

func removeFromDepsGraph(graphDeps map[string]bool, name, version string) (removedCount int) {
	dependencyToRemove := name + "@" + version
	log.Debug("Dependency to remove", dependencyToRemove)
	if version != "" {
		// If version is specifed, remove the exact match, i.e the module with a specific version
		if _, exists := graphDeps[dependencyToRemove]; exists {
			delete(graphDeps, dependencyToRemove)
			removedCount++
		}
	} else {
		// Otherwise remove any dependencies matching the module name, i.e any versions of the module
		for k := range graphDeps {
			if strings.HasPrefix(k, dependencyToRemove) {
				delete(graphDeps, k)
				removedCount++
			}
		}
	}
	return removedCount
}

func getReplaceDependencies() ([]string, error) {
	rootDir, err := cmd.GetProjectRoot()
	if err != nil {
		return nil, err
	}
	modFilePath := filepath.Join(rootDir, "go.mod")
	modFileContent, err := ioutil.ReadFile(modFilePath)
	if err != nil {
		return nil, err
	}
	return parseModForReplaceDependencies(string(modFileContent))
}

func parseModForReplaceDependencies(modFileContent string) ([]string, error) {
	replaceLinerRegExp, err := clientutils.GetRegExp(`[^\s*]?replace (?:[\(\w\.@:%_\+-.~#?&]?.+)=>(?:[\(\w\.@:%_\+-.~#?&]?.+)`)
	if err != nil {
		return nil, err
	}
	replaceLinerDependencies := replaceLinerRegExp.FindAllString(modFileContent, -1)
	replaceRegExp, err := clientutils.GetRegExp(`\s*replace\s*\(`)
	if err != nil {
		return replaceLinerDependencies, err
	}
	replaceDependencies := replaceRegExp.FindAllString(modFileContent, -1)
	if len(replaceDependencies) > 0 {
		log.Debug("Found replace block...")
		replacePosition := strings.Index(modFileContent, replaceDependencies[0])
		lines := strings.Split(modFileContent[replacePosition+len(replaceDependencies[0]):], "\n")
		for _, line := range lines {
			if line == ")" {
				break
			}
			if line == "" || line == "\n" {
				continue
			}
			replaceLinerDependencies = append(replaceLinerDependencies, line)
		}
	}
	return replaceLinerDependencies, nil
}

// Runs go mod graph command with fallback.
func getDependenciesGraphWithFallback(targetRepo string, auth auth.ServiceDetails) (map[string]bool, error) {
	dependenciesMap := map[string]bool{}
	modulesWithErrors := map[string]previousTries{}
	usedProxy := true

	for {
		// Configuring each run to use Artifactory/VCS
		err := setOrUnsetGoProxy(usedProxy, targetRepo, auth)
		if err != nil {
			return nil, err
		}

		usedProxy = !usedProxy
		dependenciesMap, err = cmd.GetDependenciesGraph("")
		if err == nil {
			break
		}

		moduleAndVersion, err := getModuleAndVersion(usedProxy, err)
		if err != nil {
			return nil, err
		}

		modulePreviousTries, ok := modulesWithErrors[moduleAndVersion]
		modulePreviousTries.setTriedFrom(usedProxy)
		if ok && modulePreviousTries.triedFromVCS && modulePreviousTries.triedFromArtifactory {
			return nil, errorutils.CheckError(errors.New(fmt.Sprintf(FailedToRetrieve+" %s "+FromBothArtifactoryAndVcs, moduleAndVersion)))
		}
		modulesWithErrors[moduleAndVersion] = modulePreviousTries
	}

	return dependenciesMap, nil
}

func setOrUnsetGoProxy(usedProxy bool, targetRepo string, auth auth.ServiceDetails) error {
	if !usedProxy {
		log.Debug("Trying download the dependencies from Artifactory...")
		return utils.SetGoProxyWithApi(targetRepo, auth)
	} else {
		log.Debug("Trying download the dependencies from the VCS...")
		return errorutils.CheckError(os.Unsetenv(utils.GOPROXY))
	}
}

func getModuleAndVersion(usedProxy bool, err error) (string, error) {
	splittedLine := strings.Split(err.Error(), ":")
	utils.LogDebug(err, usedProxy)
	if len(splittedLine) < 2 {
		return "", errorutils.CheckError(errors.New("Missing module name and version in the error message " + err.Error()))
	}
	return strings.TrimSpace(splittedLine[1]), nil
}

func populateModWithTidy(path string) error {
	err := os.Chdir(filepath.Dir(path))
	if errorutils.CheckError(err) != nil {
		return err
	}
	log.Debug("Preparing to populate mod", filepath.Dir(path))
	err = removeGoSum(path)
	utils.LogError(err)
	// Running go mod tidy command
	err = cmd.RunGoModTidy()
	if err != nil {
		return err
	}

	return nil
}

func removeGoSum(path string) error {
	// Remove go.sum file to avoid checksum conflicts with the old go.sum
	goSum := filepath.Join(filepath.Dir(path), "go.sum")
	exists, err := fileutils.IsFileExists(goSum, false)
	if err != nil {
		return err
	}
	if exists {
		err = os.Remove(goSum)
		if errorutils.CheckError(err) != nil {
			return err
		}
	}
	return nil
}

func runGoModGraph() (output map[string]bool, err error) {
	// Running go mod graph command
	return cmd.GetDependenciesGraph("")
}

type previousTries struct {
	triedFromArtifactory bool
	triedFromVCS         bool
}

func (pt *previousTries) setTriedFrom(usedProxy bool) {
	if usedProxy {
		pt.triedFromArtifactory = true
	} else {
		pt.triedFromVCS = true
	}
}

// Download the dependencies from VCS and publish them to Artifactory.
func getDependencies(dependenciesToPublish map[string]bool) (cachePath string, packageDependencies []Package, err error) {
	cachePath, err = utils.GetCachePath()
	if err != nil {
		return
	}
	packageDependencies, err = GetDependencies(cachePath, dependenciesToPublish)
	return
}
