package packagemanager

import (
	"bufio"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/d-tux/go-fstab"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func DefinePHPVersion() (string, error) {
	if _, err := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); err == nil {
		out, err := utils.ExecCommandWithOutput("chroot", chrootPHPPath, "/usr/bin/php", "--version")
		if err != nil {
			return "", errors.WithMessage(err, "failed to check php version")
		}

		return parsePHPVersion(out)
	}

	phpPath, err := exec.LookPath("php")
	if err != nil {
		return "", errors.WithMessage(err, "php command not found")
	}

	cmd := exec.Command(phpPath, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.WithMessage(err, "failed to check php version")
	}

	return parsePHPVersion(string(out))
}

func DefinePHPExtensions() ([]string, error) {
	var out string
	var err error

	if _, statErr := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); statErr == nil {
		out, err = utils.ExecCommandWithOutput(
			"chroot", chrootPHPPath, "/usr/bin/php", "-r", "echo implode(' ', get_loaded_extensions());",
		)
	} else {
		out, err = utils.ExecCommandWithOutput(
			"php", "-r", "echo implode(' ', get_loaded_extensions());",
		)
	}
	if err != nil {
		return nil, errors.WithMessage(err, "failed to exec php -r")
	}

	extensions := strings.Split(out, " ")
	for i := range extensions {
		extensions[i] = strings.ToLower(strings.TrimSpace(extensions[i]))
	}

	return extensions, nil
}

func DefinePHPCommandAndArgs(args ...string) (string, []string, error) {
	if _, statErr := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); statErr == nil {
		resultArgs := append([]string{chrootPHPPath, "/usr/bin/php"}, args...)

		return "chroot", resultArgs, nil
	}

	return "php", args, nil
}

func IsPHPCommandAvailable(_ context.Context) bool {
	return utils.IsCommandAvailable("php") || isChrootPHPAvailable()
}

func isChrootPHPAvailable() bool {
	_, err := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile))
	if err != nil {
		return false
	}

	_, err = os.Stat(filepath.Join(chrootPHPPath, "usr/bin/php"))

	return err == nil
}

var bindMu = sync.Mutex{}

//nolint:funlen
func TryBindPHPDirectories(_ context.Context, source string) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	if _, err := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); err != nil {
		// No chroot php package
		//nolint:nilerr
		return nil
	}

	bindMu.Lock()
	defer bindMu.Unlock()

	dest := filepath.Join(chrootPHPPath, source)
	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(dest, 0755)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	mounts, err := fstab.ParseProc()
	if err != nil {
		return errors.WithMessage(err, "failed to parse proc")
	}

	for _, mount := range mounts {
		if mount.File == dest {
			// Already mounted
			return nil
		}
	}

	mounts, err = fstab.ParseSystem()
	if err != nil {
		return errors.WithMessage(err, "failed to parse fstab")
	}

	for _, mount := range mounts {
		if mount.File == dest {
			err = utils.ExecCommand("mount", dest)
			if err != nil {
				return err
			}

			return nil
		}
	}

	m := fstab.Mount{
		Spec:    source,
		File:    dest,
		VfsType: "none",
		MntOps: map[string]string{
			"defaults": "",
			"bind":     "",
		},
	}

	log.Println("Updating fstab")
	err = utils.AppendContentsToFile(append([]byte{'\n'}, []byte(m.String())...), "/etc/fstab")
	if err != nil {
		return errors.WithMessage(err, "failed to append contents to fstab")
	}

	err = utils.ExecCommand("mount", dest)
	if err != nil {
		return err
	}

	return nil
}

func parsePHPVersion(s string) (string, error) {
	version := ""

	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		data := strings.Split(scanner.Text(), " ")
		if len(data) < 2 || !strings.HasPrefix(data[0], "PHP") {
			continue
		}
		parsedVersion := strings.Split(data[1], "-")
		if len(parsedVersion) < 1 {
			continue
		}

		vr := strings.Split(parsedVersion[0], ".")
		if len(vr) < 2 {
			return "", errors.New("failed to parse php version: empty version")
		}

		version = vr[0] + "." + vr[1]

		break
	}

	if version == "" {
		return "", errors.New("failed to parse php version")
	}

	return version, nil
}
