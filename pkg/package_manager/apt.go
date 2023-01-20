package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os/exec"
	"strconv"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/pkg/errors"
)

// https://github.com/arduino/go-apt-client/blob/master/apt.go

type APT struct{}

// Search list packages available in the system that match the search
// pattern.
func (apt *APT) Search(_ context.Context, pattern string) ([]*Package, error) {
	cmd := exec.Command(
		"dpkg-query",
		"-W",
		"-f=${Package}\t${Architecture}\t${db:Status-Status}\t${Version}\t${Installed-Size}\t${Binary:summary}\n",
		pattern,
	)

	out, err := cmd.CombinedOutput()
	log.Print(string(out))
	if err != nil {
		// Avoid returning an error if the list is empty
		if bytes.Contains(out, []byte("no packages found matching")) {
			return []*Package{}, nil
		}
		return nil, errors.WithMessage(err, "failed to run dpkg-query")
	}

	return parseDpkgQueryOutput(out), nil
}

func parseDpkgQueryOutput(out []byte) []*Package {
	res := []*Package{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		data := strings.Split(scanner.Text(), "\t")
		size, err := strconv.Atoi(data[4])
		if err != nil {
			// Ignore error
			size = 0
		}
		res = append(res, &Package{
			Name:             data[0],
			Architecture:     data[1],
			Status:           data[2],
			Version:          data[3],
			InstalledSizeKB:  size,
			ShortDescription: data[5],
		})
	}
	return res
}

// CheckForUpdates runs an apt update to retrieve new packages available
// from the repositories.
func (apt *APT) CheckForUpdates(_ context.Context) error {
	cmd := exec.Command("apt-get", "update", "-q")
	log.Println(cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	return cmd.Run()
}

// Install installs a set of packages.
func (apt *APT) Install(_ context.Context, packs ...string) error {
	args := []string{"install", "-y"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("apt-get", args...)
	log.Println(cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	return cmd.Run()
}

// Remove removes a set of packages.
func (apt *APT) Remove(_ context.Context, packs ...string) error {
	args := []string{"remove", "-y"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("apt-get", args...)
	log.Println(cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	return cmd.Run()
}

type ExtendedAPT struct {
	apt *APT
}

func NewExtendedAPT(apt *APT) *ExtendedAPT {
	return &ExtendedAPT{
		apt: apt,
	}
}

func (e *ExtendedAPT) Search(ctx context.Context, name string) ([]*Package, error) {
	return e.apt.Search(ctx, name)
}

func (e *ExtendedAPT) Install(ctx context.Context, packs ...string) error {
	// apt.Install(ctx, "software-properties-common", "apt-transport-https")

	packs = e.replaceAliases(ctx, packs)

	err := e.preInstallationSteps(ctx, packs...)
	if err != nil {
		return err
	}

	return e.apt.Install(ctx, packs...)
}

func (e *ExtendedAPT) CheckForUpdates(ctx context.Context) error {
	return e.apt.CheckForUpdates(ctx)
}

func (e *ExtendedAPT) Remove(ctx context.Context, packs ...string) error {
	packs = e.replaceAliases(ctx, packs)
	return e.apt.Remove(ctx, packs...)
}

var packageAliases = map[string]map[string]map[string]string{
	"debian": {
		"squeeze": {
			MySQLServerPackage: "mysql-server",
			Lib32GCCPackage:    "lib32gcc1",
		},
		"wheezy": {
			MySQLServerPackage: "mysql-server",
			Lib32GCCPackage:    "lib32gcc1",
		},
		"jessie": {
			MySQLServerPackage: "mysql-server",
			Lib32GCCPackage:    "lib32gcc1",
		},
		"stretch": {
			MySQLServerPackage: "default-mysql-server",
			Lib32GCCPackage:    "lib32gcc1",
		},
		"buster": {
			MySQLServerPackage: "default-mysql-server",
			Lib32GCCPackage:    "lib32gcc1",
		},
		"bullseye": {
			MySQLServerPackage: "default-mysql-server",
			Lib32GCCPackage:    "lib32gcc-s1",
		},
		"sid": {
			MySQLServerPackage: "default-mysql-server",
			Lib32GCCPackage:    "lib32gcc-s1",
		},
	},
	"ubuntu": {
		"jammy": {
			Lib32GCCPackage: "lib32gcc-s1",
		},
		"kinetic": {
			Lib32GCCPackage: "lib32gcc-s1",
		},
		"lunar": {
			Lib32GCCPackage: "lib32gcc-s1",
		},
	},
}

func (e *ExtendedAPT) replaceAliases(ctx context.Context, packs []string) []string {
	replacedPacks := make([]string, 0, len(packs))

	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if alias, exists := packageAliases[osInfo.Distribution][osInfo.DistributionCodename][pack]; exists {
			replacedPacks = append(replacedPacks, alias)
		} else {
			replacedPacks = append(replacedPacks, pack)
		}
	}

	return replacedPacks
}

// nolint
func (e *ExtendedAPT) preInstallationSteps(_ context.Context, packs ...string) error {
	for _, pack := range packs {
		// nolint
		if pack == PHPPackage {
			//return e.apt.Search(ctx, "software-properties-common")
		}
	}

	return nil
}
