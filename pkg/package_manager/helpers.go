package packagemanager

import (
	"bufio"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func DefinePHPVersion() (string, error) {
	cmd := exec.Command("php", "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.WithMessage(err, "failed to check php version")
	}

	return parsePHPVersion(string(out))
}

func parsePHPVersion(s string) (string, error) {
	version := ""

	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		data := strings.Split(scanner.Text(), " ")
		if len(data) < 2 {
			continue
		}
		if data[0] != "PHP" {
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
	}

	if version == "" {
		return "", errors.New("failed to parse php version")
	}

	return version, nil
}
