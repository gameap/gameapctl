package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
)

type yum struct{}

func (y *yum) Search(_ context.Context, name string) ([]PackageInfo, error) {
	cmd := exec.Command("yum", "info", name)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	log.Print(string(out))
	if err != nil {
		if bytes.Contains(out, []byte("Error: No matching Packages to list")) {
			return []PackageInfo{}, nil
		}

		return nil, err
	}

	return parseYumInfoOutput(out)
}

func parseYumInfoOutput(out []byte) ([]PackageInfo, error) {
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var packages []PackageInfo
	var currentPackage *PackageInfo

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Name":
			if currentPackage != nil {
				packages = append(packages, *currentPackage)
			}
			currentPackage = &PackageInfo{}

			currentPackage.Name = value
		case "Version":
			currentPackage.Version = value
		case "Architecture":
			currentPackage.Architecture = value
		case "Size":
			currentPackage.Size = value
		case "Description":
			currentPackage.Description = value
		case "":
			if value != "" && currentPackage != nil {
				currentPackage.Description += " " + value
			}
		}
	}

	if currentPackage != nil {
		packages = append(packages, *currentPackage)
	}

	return packages, nil
}

func (y *yum) Install(_ context.Context, packs ...string) error {
	args := []string{"install", "-y"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("yum", args...)

	cmd.Env = os.Environ()

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (y *yum) CheckForUpdates(_ context.Context) error {
	return nil
}

func (y *yum) Remove(_ context.Context, packs ...string) error {
	args := []string{"remove", "-y"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("yum", args...)

	cmd.Env = os.Environ()

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (y *yum) Purge(ctx context.Context, packs ...string) error {
	return y.Remove(ctx, packs...)
}
