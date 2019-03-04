package executers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jfrog/gocmd/cache"
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Represents go dependency when running with go-recursive-publish set to true.
type PackageWithDeps struct {
	Dependency             *Package
	transitiveDependencies []PackageWithDeps
	regExp                 *utils.RegExp
	runGoModCommand        bool
	shouldRevertToEmptyMod bool
	cachePath              string
	GoModEditMessage       string
	originalModContent     []byte
}

// Populates and publish the dependencies.
func RecursivePublish(targetRepo, goModEditMessage string, serviceManager *artifactory.ArtifactoryServicesManager) error {
	err := fileutils.CreateTempDirPath()
	if err != nil {
		return err
	}
	defer fileutils.RemoveTempDir()
	pwd := &PackageWithDeps{GoModEditMessage: goModEditMessage}
	err = pwd.Init()
	if err != nil {
		return err
	}
	collectDependenciesAndPublish(targetRepo, false, pwd, serviceManager)
	return nil
}

// Creates a new dependency
func (pwd *PackageWithDeps) New(cachePath string, dependency Package) GoPackage {
	pwd.Dependency = &dependency
	pwd.cachePath = cachePath
	pwd.transitiveDependencies = nil
	return pwd
}

// Populate the mod file and publish the dependency and it's transitive dependencies to Artifactory
func (pwd *PackageWithDeps) PopulateModAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	var path string
	log.Debug("Starting to work on", pwd.Dependency.GetId())
	serviceManager.GetConfig().GetArtDetails()
	dependenciesMap := cache.GetMap()
	published, _ := dependenciesMap[pwd.Dependency.GetId()]
	if published {
		log.Debug("Overwriting the mod file in the cache from the one from Artifactory", pwd.Dependency.GetId())
		moduleAndVersion := strings.Split(pwd.Dependency.GetId(), ":")
		client, err := httpclient.ClientBuilder().Build()
		if err != nil {
			return err
		}
		path = downloadModFileFromArtifactoryToLocalCache(pwd.cachePath, targetRepo, moduleAndVersion[0], moduleAndVersion[1], serviceManager.GetConfig().GetArtDetails(), client)
		err = pwd.updateModContent(path, cache)
		utils.LogError(err)
	}

	// Checks if mod is empty, need to run go mod tidy command to populate the mod file.
	log.Debug(fmt.Sprintf("Dependency %s mod file is empty: %t", pwd.Dependency.GetId(), !pwd.PatternMatched(pwd.regExp.GetNotEmptyModRegex())))

	// Creates the dependency in the temp folder and runs go commands: go mod tidy and go mod graph.
	// Returns the path to the project in the temp and the a map with the project dependencies
	path, output, err := pwd.createDependencyAndPrepareMod(cache)
	utils.LogError(err)
	pwd.publishDependencyAndPopulateTransitive(path, targetRepo, output, cache, serviceManager)
	return nil
}

// Updating the new mod content
func (pwd *PackageWithDeps) updateModContent(path string, cache *cache.DependenciesCache) error {
	if path != "" {
		modContent, err := ioutil.ReadFile(path)
		if err != nil {
			cache.IncrementFailures()
			return errorutils.CheckError(err)
		}
		pwd.Dependency.SetModContent(modContent)
	}
	return nil
}

// Init the dependency information if needed.
func (pwd *PackageWithDeps) Init() error {
	var err error
	pwd.regExp, err = utils.GetRegex()
	if err != nil {
		return err
	}
	return nil
}

// Returns true if regex found a match otherwise false.
func (pwd *PackageWithDeps) PatternMatched(regExp *regexp.Regexp) bool {
	lines := strings.Split(string(pwd.Dependency.modContent), "\n")
	for _, line := range lines {
		if regExp.FindString(line) != "" {
			return true
		}
	}
	return false
}

// Creates the dependency in the temp folder and runs go mod tidy and go mod graph
// Returns the path to the project in the temp and the a map with the project dependencies
func (pwd *PackageWithDeps) createDependencyAndPrepareMod(cache *cache.DependenciesCache) (path string, output map[string]bool, err error) {
	path, err = pwd.getModPathAndUnzipDependency(path)
	if err != nil {
		return
	}
	pwd.shouldRevertToEmptyMod = false
	// Check the mod in the cache if empty or not
	if pwd.PatternMatched(pwd.regExp.GetNotEmptyModRegex()) {
		err = pwd.useCachedMod(path)
		if err != nil {
			return
		}
	} else {
		published, _ := cache.GetMap()[pwd.Dependency.GetId()]
		if !published {
			pwd.prepareUnpublishedDependency(path)
		} else {
			pwd.prepareResolvedDependency(path)
		}
	}
	output, err = runGoModGraph(path)
	return
}

func (pwd *PackageWithDeps) prepareResolvedDependency(path string) {
	// Put the mod file to temp
	err := writeModContentToModFile(path, pwd.Dependency.GetModContent())
	utils.LogError(err)
	// If not empty --> use the mod file and don't run go mod tidy
	// If empty --> Run go mod tidy. Publish the package with empty mod file.
	if !pwd.PatternMatched(pwd.regExp.GetNotEmptyModRegex()) {
		log.Debug("The mod still empty after downloading from Artifactory:", pwd.Dependency.GetId())
		originalModContent := pwd.Dependency.GetModContent()
		pwd.prepareAndRunTidy(path, originalModContent)
	} else {
		log.Debug("Project mod file is not empty after downloading from Artifactory", pwd.Dependency.id)
	}
}

func (pwd *PackageWithDeps) prepareAndRunTidy(path string, originalModContent []byte) {
	err := populateModWithTidy(path)
	utils.LogError(err)
	pwd.originalModContent = originalModContent
}

func (pwd *PackageWithDeps) prepareUnpublishedDependency(path string) {
	// Put the mod file to temp
	err := writeModContentToModFile(path, pwd.Dependency.GetModContent())
	utils.LogError(err)
	// If not empty --> use the mod file and don't run go mod tidy
	// If empty --> Run go mod tidy. Publish the package with empty mod file.
	if !pwd.PatternMatched(pwd.regExp.GetNotEmptyModRegex()) {
		log.Debug("The mod still empty after downloading from VCS:", pwd.Dependency.GetId())
		originalModContent := pwd.Dependency.GetModContent()
		pwd.prepareAndRunTidy(path, originalModContent)
	} else {
		log.Debug("Project mod file is not empty after downloading from VCS", pwd.Dependency.id)
	}
}

func (pwd *PackageWithDeps) useCachedMod(path string) error {
	// Mod not empty in the cache. Use it.
	log.Debug("Using the mod in the cache since not empty:", pwd.Dependency.GetId())
	err := writeModContentToModFile(path, pwd.Dependency.GetModContent())
	utils.LogError(err)
	err = os.Chdir(filepath.Dir(path))
	if errorutils.CheckError(err) != nil {
		return err
	}
	utils.LogError(removeGoSum(path))
	return nil
}

func (pwd *PackageWithDeps) getModPathAndUnzipDependency(path string) (string, error) {
	err := os.Unsetenv(utils.GOPROXY)
	if err != nil {
		return "", err
	}
	// Unzips the zip file into temp
	tempDir, err := createDependencyInTemp(pwd.Dependency.GetZipPath())
	if err != nil {
		return "", err
	}
	path = pwd.getModPathInTemp(tempDir)
	return path, err
}

func (pwd *PackageWithDeps) prepareAndRunInit(pathToModFile string) error {
	log.Debug("Preparing to init", pathToModFile)
	err := os.Chdir(filepath.Dir(pathToModFile))
	if errorutils.CheckError(err) != nil {
		return err
	}
	exists, err := fileutils.IsFileExists(pathToModFile, false)
	utils.LogError(err)
	if exists {
		err = os.Remove(pathToModFile)
		utils.LogError(err)
	}
	// Mod empty.
	// If empty, run go mod init
	moduleId := pwd.Dependency.GetId()
	moduleInfo := strings.Split(moduleId, ":")
	return cmd.RunGoModInit(goModDecode(moduleInfo[0]))
}

func writeModContentToModFile(path string, modContent []byte) error {
	return ioutil.WriteFile(path, modContent, 0700)
}

func (pwd *PackageWithDeps) getModPathInTemp(tempDir string) string {
	moduleId := pwd.Dependency.GetId()
	moduleInfo := strings.Split(moduleId, ":")
	moduleInfo[0] = goModDecode(moduleInfo[0])
	if len(moduleInfo) > 1 {
		moduleInfo[1] = goModDecode(moduleInfo[1])
	}
	moduleId = strings.Join(moduleInfo, ":")
	modulePath := strings.Replace(moduleId, ":", "@", 1)
	path := filepath.Join(tempDir, modulePath, "go.mod")
	return path
}

func (pwd *PackageWithDeps) publishDependencyAndPopulateTransitive(pathToModFile, targetRepo string, graphDependencies map[string]bool, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	// If the mod is not empty, populate transitive dependencies
	if len(graphDependencies) > 0 {
		sumFileContent, sumFileStat, err := cmd.GetSumContentAndRemove(filepath.Dir(pathToModFile))
		utils.LogError(err)
		pwd.setTransitiveDependencies(targetRepo, graphDependencies, cache, serviceManager.GetConfig().GetArtDetails())
		if len(sumFileContent) > 0 && sumFileStat != nil {
			cmd.RestoreSumFile(filepath.Dir(pathToModFile), sumFileContent, sumFileStat)
		}
	}

	published, _ := cache.GetMap()[pwd.Dependency.GetId()]
	// Populate and publish the transitive dependencies.
	if pwd.transitiveDependencies != nil {
		pwd.populateTransitive(targetRepo, cache, serviceManager)
	}

	if !published && pwd.shouldRevertToEmptyMod {
		log.Debug("Reverting to the original mod of", pwd.Dependency.GetId())
		pwd.Dependency.SetModContent(pwd.originalModContent)
		err := pwd.writeModContentToGoCache()
		utils.LogError(err)
	}
	// Publish to Artifactory the dependency if needed.
	if !published {
		err := pwd.prepareAndPublish(targetRepo, cache, serviceManager)
		utils.LogError(err)
	}

	// Remove from temp folder the dependency.
	err := os.RemoveAll(filepath.Dir(pathToModFile))
	if errorutils.CheckError(err) != nil {
		log.Error(fmt.Sprintf("Removing the following directory %s has encountred an error: %s", err, filepath.Dir(pathToModFile)))
	}

	return nil
}

// Prepare for publishing and publish the dependency to Artifactory
// Mark this dependency as published
func (pwd *PackageWithDeps) prepareAndPublish(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) error {
	err := pwd.Dependency.prepareAndPublish(targetRepo, cache, serviceManager)
	cache.GetMap()[pwd.Dependency.GetId()] = true
	return err
}

func (pwd *PackageWithDeps) setTransitiveDependencies(targetRepo string, graphDependencies map[string]bool, cache *cache.DependenciesCache, auth auth.ArtifactoryDetails) {
	var dependencies []PackageWithDeps
	for transitiveDependency := range graphDependencies {
		module := strings.Split(transitiveDependency, "@")
		if len(module) == 2 {
			dependenciesMap := cache.GetMap()
			name := goModEncode(module[0])
			version := goModEncode(module[1])
			_, exists := dependenciesMap[name+":"+version]
			if !exists {
				// Check if the dependency is in the local cache.
				dep, err := createDependency(pwd.cachePath, name, version)
				utils.LogError(err)
				if err != nil {
					continue
				}
				// Check if this dependency exists in Artifactory.
				client, err := httpclient.ClientBuilder().Build()
				if err != nil {
					continue
				}
				downloadedFromArtifactory, err := shouldDownloadFromArtifactory(module[0], module[1], targetRepo, auth, client)
				utils.LogError(err)
				if err != nil {
					continue
				}
				if dep == nil {
					// Dependency is missing in the local cache. Need to download it...
					dep, err = downloadAndCreateDependency(pwd.cachePath, name, version, transitiveDependency, targetRepo, downloadedFromArtifactory, auth)
					utils.LogError(err)
					if err != nil {
						continue
					}
				}

				if dep != nil {
					log.Debug(fmt.Sprintf("Dependency %s has transitive dependency %s", pwd.Dependency.GetId(), dep.GetId()))
					depsWithTrans := &PackageWithDeps{Dependency: dep,
						regExp:           pwd.regExp,
						cachePath:        pwd.cachePath,
						GoModEditMessage: pwd.GoModEditMessage}
					dependencies = append(dependencies, *depsWithTrans)
					dependenciesMap[name+":"+version] = downloadedFromArtifactory
				}
			} else {
				log.Debug("Dependency", transitiveDependency, "has been previously added.")
			}
		}
	}
	pwd.transitiveDependencies = dependencies
}

func (pwd *PackageWithDeps) writeModContentToGoCache() error {
	moduleAndVersion := strings.Split(pwd.Dependency.GetId(), ":")
	pathToModule := strings.Split(moduleAndVersion[0], "/")
	path := filepath.Join(pwd.cachePath, strings.Join(pathToModule, fileutils.GetFileSeparator()), "@v", moduleAndVersion[1]+".mod")
	err := ioutil.WriteFile(path, pwd.Dependency.GetModContent(), 0700)
	return errorutils.CheckError(err)
}

// Runs over the transitive dependencies, populate the mod files of those transitive dependencies
func (pwd *PackageWithDeps) populateTransitive(targetRepo string, cache *cache.DependenciesCache, serviceManager *artifactory.ArtifactoryServicesManager) {
	cache.IncrementTotal(len(pwd.transitiveDependencies))
	for _, transitiveDep := range pwd.transitiveDependencies {
		published, _ := cache.GetMap()[transitiveDep.Dependency.GetId()]
		if !published {
			log.Debug("Starting to work on transitive dependency:", transitiveDep.Dependency.GetId())
			transitiveDep.PopulateModAndPublish(targetRepo, cache, serviceManager)
		} else {
			cache.IncrementSuccess()
			log.Debug("The dependency", transitiveDep.Dependency.GetId(), "was already handled")
		}
	}
}

func (pwd *PackageWithDeps) signModFile() {
	log.Debug("Signing mod file for", pwd.Dependency.GetId())
	newContent := append([]byte(pwd.GoModEditMessage+"\n\n"), pwd.Dependency.GetModContent()...)
	pwd.Dependency.SetModContent(newContent)
}
