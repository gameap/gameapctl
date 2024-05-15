package apt

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
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

	log.Println("All packages: ", packages)

	return aptSearch(q, packages, false)
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
	for _, packagesFile := range allPackagesFiles {
		if strings.HasSuffix(packagesFile.Name(), "_Packages") {
			matchingPackagesFiles = append(matchingPackagesFiles, packagesFile.Name())
		}
	}

	return matchingPackagesFiles, nil
}

// buildPackagesList: return packages available from a list of repositories.
//
//nolint:funlen
func buildPackagesList(repoList []string) ([]Package, error) {
	var packageList []Package
	for _, packagesFile := range repoList {
		f, errOpen := os.Open(filepath.Join(aptListPath, packagesFile))
		if errOpen != nil {
			return nil, errOpen
		}
		defer func() {
			err := f.Close()
			if err != nil {
				log.Println("failed to close file", err)
			}
		}()

		scanner := bufio.NewScanner(f)

		var p Package

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				if p.PackageName != "" {
					packageList = append(packageList, p)
					p = Package{}
				}

				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Package":
				p.PackageName = value
			case "Version":
				p.Version = value
			case "Architecture":
				p.Architecture = value
			case "Depends":
				p.Depends = strings.Split(value, ",")
			case "Size":
				p.Size = value
			case "Installed-Size":
				p.InstalledSize = value
			case "Description":
				p.Description = value
			case "Section":
				p.Section = value
			case "MD5sum":
				p.Md5sum = value
			case "SHA256":
				p.Sha256 = value
			}
		}
	}

	return packageList, nil
}
