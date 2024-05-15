package apt

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// This is modified package from https://github.com/Sfrisio/go-apt-search

const aptListPath = "/var/lib/apt/lists/"

type Package struct {
	PackageName   string
	Version       string
	Architecture  string
	Depends       []string
	Size          string
	InstalledSize string
	Description   string
	Section       string
	Md5sum        string
	Sha256        string
}

type RepoArchive struct {
	Domain       string
	Distribution string
	Area         string
	Architecture string
	ListFileName string
}

func Search(q string) ([]Package, error) {
	packages, err := aptListAllPackages()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to list all packages")
	}

	return aptSearch(q, packages, true)
}

// aptSearch allows to perform a targeted search using the exact name of the package to be searched,
// or a keyword search that will result in all packages that include that string in the name.
func aptSearch(searchPackage string, packagesList []Package, searchExactName bool) ([]Package, error) {
	var filteredPackageList []Package
	for _, singlePackage := range packagesList {
		if searchExactName {
			if singlePackage.PackageName == searchPackage {
				filteredPackageList = append(filteredPackageList, singlePackage)
			}
		} else {
			if strings.Contains(singlePackage.PackageName, searchPackage) {
				filteredPackageList = append(filteredPackageList, singlePackage)
			}
		}
	}

	return filteredPackageList, nil
}

// AptListALL scan the all source.list on the system and return the list of all available packages.
func aptListAllPackages() ([]Package, error) {
	allPackagesFiles, errGetRepoFileList := getRepoFileList()
	if errGetRepoFileList != nil {
		return nil, errGetRepoFileList
	}
	allPackagesList, errBuildPackagesList := buildPackagesList(allPackagesFiles)
	if errBuildPackagesList != nil {
		return nil, errBuildPackagesList
	}

	return allPackagesList, nil
}

// getRepoFileList: read files from /var/lib/apt/lists and return only packages
//
// I preferred to use os.ReadDir instead of filepath.Walk because I am not interested in the list of files in the partial directory.
//
//nolint:lll
func getRepoFileList() ([]string, error) {
	allPackagesFiles, errReadDir := os.ReadDir(aptListPath)
	if errReadDir != nil {
		return nil, errReadDir
	}
	var matchingPackagesFiles []string
	filterPackagesFile := regexp.MustCompile(`.*\_Packages$`)
	for _, packagesFile := range allPackagesFiles {
		if filterPackagesFile.MatchString(packagesFile.Name()) {
			matchingPackagesFiles = append(matchingPackagesFiles, packagesFile.Name())
		}
	}

	return matchingPackagesFiles, nil
}

// buildPackagesList: return packages available from a list of repositories.
//
//nolint:funlen
func buildPackagesList(repoList []string) ([]Package, error) {
	var packagesList []Package
	for _, packagesFile := range repoList {
		readPackageFile, errOpen := os.ReadFile(filepath.Join(aptListPath, packagesFile))
		if errOpen != nil {
			return nil, errOpen
		}
		lines := strings.Split(string(readPackageFile), "\n")
		var packageNameFromList string
		var versionFromList string
		var architectureFromList string
		var dependsFromList []string
		var sizeFromList string
		var installedSizeFromList string
		var descriptionFromList string
		var sectionFromList string
		var md5sumFromList string
		var sha256FromList string
		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "Package:"):
				packageNameFromList, _ = strings.CutPrefix(line, "Package:")
			case strings.HasPrefix(line, "Version:"):
				versionFromList, _ = strings.CutPrefix(line, "Version:")
			case strings.HasPrefix(line, "Architecture:"):
				architectureFromList, _ = strings.CutPrefix(line, "Architecture:")
			case strings.HasPrefix(line, "Depends:"):
				dependsList, _ := strings.CutPrefix(line, "Depends:")
				dependsFromList = strings.Split(dependsList, ",")
			case strings.HasPrefix(line, "Description:"):
				descriptionFromList, _ = strings.CutPrefix(line, "Description:")
			case strings.HasPrefix(line, "Size:"):
				sizeFromList, _ = strings.CutPrefix(line, "Size:")
			case strings.HasPrefix(line, "Installed-Size:"):
				installedSizeFromList, _ = strings.CutPrefix(line, "Installed-Size:")
			case strings.HasPrefix(line, "Section:"):
				sectionFromList, _ = strings.CutPrefix(line, "Section:")
			case strings.HasPrefix(line, "MD5sum:"):
				md5sumFromList, _ = strings.CutPrefix(line, "MD5sum:")
			case strings.HasPrefix(line, "SHA256:"):
				sha256FromList, _ = strings.CutPrefix(line, "SHA256:")
			case line == "":
				// information dump because each new line starts a new package
				packagesList = append(packagesList, Package{
					PackageName:   strings.TrimSpace(packageNameFromList),
					Version:       strings.TrimSpace(versionFromList),
					Architecture:  strings.TrimSpace(architectureFromList),
					Depends:       dependsFromList,
					Size:          strings.TrimSpace(sizeFromList),
					InstalledSize: strings.TrimSpace(installedSizeFromList),
					Description:   strings.TrimSpace(descriptionFromList),
					Section:       strings.TrimSpace(sectionFromList),
					Md5sum:        strings.TrimSpace(md5sumFromList),
					Sha256:        strings.TrimSpace(sha256FromList),
				})
			}
		}
	}

	return packagesList, nil
}
