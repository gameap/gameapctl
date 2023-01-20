package osinfo

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/matishsiao/goInfo"
	"github.com/pkg/errors"
)

type Info struct {
	Kernel               string
	Core                 string
	Distribution         string
	DistributionVersion  string
	DistributionCodename string
	Platform             string
	OS                   string
	Hostname             string
	CPUs                 int
}

func GetOSInfo() (Info, error) {
	gi, err := goInfo.GetInfo()
	if err != nil {
		return Info{}, err
	}

	return Info{
		Kernel:   gi.Kernel,
		Core:     gi.Core,
		Platform: gi.Platform,
		OS:       gi.OS,
		Hostname: gi.Hostname,
		CPUs:     gi.CPUs,
	}, nil
}

//nolint:unused
type osInfo struct {
	OS   string
	Dist string
}

// nolint
func detectOS() (osInfo, error) {
	const (
		etcLsbRelease = "/etc/lsb-release"
		etcOsRelease  = "/etc/os-release"
	)

	result := osInfo{}

	if _, err := os.Stat(etcLsbRelease); !os.IsNotExist(err) {
		// /etc/lsb-release exists, read it
		data, err := os.ReadFile(etcLsbRelease)
		if err != nil {
			panic(err)
		}

		// extract ID and VERSION_ID from /etc/lsb-release
		id := extractField(data, "ID")
		versionID := extractField(data, "VERSION_ID")

		if id == "raspbian" {
			// raspbian
			result.OS = id
			result.Dist = versionID
		} else {
			// debian
			result.OS = extractField(data, "DISTRIB_ID")
			result.Dist = extractField(data, "DISTRIB_CODENAME")
			if result.Dist == "" {
				result.Dist = extractField(data, "DISTRIB_RELEASE")
			}
		}
	} else if _, err := os.Stat(etcOsRelease); !os.IsNotExist(err) {
		// /etc/os-release exists, read it
		data, err := os.ReadFile(etcOsRelease)
		if err != nil {
			panic(err)
		}

		// extract ID and VERSION_CODENAME from /etc/os-release
		id := extractField(data, "ID")
		versionCodename := extractField(data, "VERSION_CODENAME")

		if id == "" {
			// fallback to /etc/lsb-release
			result.OS = extractField(data, "DISTRIB_ID")
			result.Dist = extractField(data, "DISTRIB_CODENAME")
			if result.Dist == "" {
				result.Dist = extractField(data, "DISTRIB_RELEASE")
			}
		} else {
			result.OS = id
			if versionCodename != "" {
				result.Dist = versionCodename
			} else {
				result.Dist = extractField(data, "VERSION_ID")
			}
		}
	} else if _, err := exec.LookPath("lsb_release"); err == nil {
		// lsb_release exists
		// extract dist from lsb_release -c
		cmd := exec.Command("lsb_release", "-c")
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			panic(err)
		}
		result.Dist = strings.Split(string(out), ":")[1]
		result.Dist = strings.TrimSpace(result.Dist)

		// extract os from lsb_release -i
		cmd = exec.Command("lsb_release", "-i")
		cmd.Stderr = os.Stderr
		out, err = cmd.Output()
		if err != nil {
			panic(err)
		}
		result.OS = strings.Split(string(out), ":")[1]
		result.OS = strings.TrimSpace(result.OS)
		result.OS = strings.ToLower(result.OS)
	}

	_, debianVersionErr := os.Stat("/etc/debian_version")
	if result.Dist == "" && !errors.Is(debianVersionErr, os.ErrNotExist) {

		// /etc/debian_version exists
		// extract os from /etc/issue
		data, err := os.ReadFile("/etc/issue")
		if err != nil {
			panic(err)
		}
		result.OS = strings.Split(string(data), " ")[0]
		result.OS = strings.TrimSpace(result.OS)
		result.OS = strings.ToLower(result.OS)

		// extract dist from /etc/debian_version
		data, err = os.ReadFile("/etc/debian_version")
		if err != nil {
			panic(err)
		}
		result.Dist = strings.Split(string(data), "/")[0]
		result.Dist = strings.TrimSpace(result.Dist)
	}

	if result.Dist == "" {
		// unknown os
		panic("unknown operating system")
	}

	// cleanup
	result.OS = strings.ReplaceAll(result.OS, " ", "")
	result.Dist = strings.ReplaceAll(result.Dist, " ", "")

	// lowercase
	result.OS = strings.ToLower(result.OS)
	result.Dist = strings.ToLower(result.Dist)

	return result, nil
}

//nolint:unused
func extractField(data []byte, key string) string {
	regex := regexp.MustCompile(fmt.Sprintf(`(?m)^%s=([^\s]+)`, key))
	matches := regex.FindStringSubmatch(string(data))
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}
