package osinfo

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
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

	result := Info{
		Kernel:   gi.Kernel,
		Core:     gi.Core,
		Platform: gi.Platform,
		OS:       gi.OS,
		Hostname: gi.Hostname,
		CPUs:     gi.CPUs,
	}

	if result.Platform == "" || result.Platform == "unknown" {
		result.Platform = runtime.GOARCH
	}

	if gi.OS == "GNU/Linux" {
		info, err := detectLinuxDist()
		if err != nil {
			return result, err
		}
		result.Distribution = info.Name
		result.DistributionVersion = info.Version
		result.DistributionCodename = info.VersionCodename
	} else {
		result.Distribution = gi.OS
		result.DistributionVersion = gi.Kernel
		result.DistributionCodename = gi.Kernel
	}

	return result, nil
}

type distInfo struct {
	Name            string
	Version         string
	VersionCodename string
}

//nolint:funlen,unparam
func detectLinuxDist() (distInfo, error) {
	const (
		etcLsbRelease = "/etc/lsb-release"
		etcOsRelease  = "/etc/os-release"
	)

	result := distInfo{}

	//nolint:nestif
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
			result.Name = id
			result.VersionCodename = versionID
		} else {
			// debian
			result.Name = extractField(data, "DISTRIB_ID")
			result.VersionCodename = extractField(data, "DISTRIB_CODENAME")
			if result.VersionCodename == "" {
				result.VersionCodename = extractField(data, "DISTRIB_RELEASE")
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
			result.Name = extractField(data, "DISTRIB_ID")
			result.VersionCodename = extractField(data, "DISTRIB_CODENAME")
			if result.VersionCodename == "" {
				result.VersionCodename = extractField(data, "DISTRIB_RELEASE")
			}
		} else {
			result.Name = id
			if versionCodename != "" {
				result.VersionCodename = versionCodename
			} else {
				result.VersionCodename = extractField(data, "VERSION_ID")
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
		result.VersionCodename = strings.Split(string(out), ":")[1]
		result.VersionCodename = strings.TrimSpace(result.VersionCodename)

		// extract os from lsb_release -i
		cmd = exec.Command("lsb_release", "-i")
		cmd.Stderr = os.Stderr
		out, err = cmd.Output()
		if err != nil {
			panic(err)
		}
		result.Name = strings.Split(string(out), ":")[1]
		result.Name = strings.TrimSpace(result.Name)
		result.Name = strings.ToLower(result.Name)
	}

	_, debianVersionErr := os.Stat("/etc/debian_version")
	if result.VersionCodename == "" && !errors.Is(debianVersionErr, os.ErrNotExist) {
		// /etc/debian_version exists
		// extract os from /etc/issue
		data, err := os.ReadFile("/etc/issue")
		if err != nil {
			panic(err)
		}
		result.Name = strings.Split(string(data), " ")[0]
		result.Name = strings.TrimSpace(result.Name)
		result.Name = strings.ToLower(result.Name)

		// extract dist from /etc/debian_version
		data, err = os.ReadFile("/etc/debian_version")
		if err != nil {
			panic(err)
		}
		result.VersionCodename = strings.Split(string(data), "/")[0]
		result.VersionCodename = strings.TrimSpace(result.VersionCodename)
	}

	if result.VersionCodename == "" {
		// unknown os
		panic("unknown operating system")
	}

	// cleanup
	result.Name = strings.ReplaceAll(result.Name, " ", "")
	result.VersionCodename = strings.ReplaceAll(result.VersionCodename, " ", "")

	// lowercase
	result.Name = strings.ToLower(result.Name)
	result.VersionCodename = strings.ToLower(result.VersionCodename)

	return result, nil
}

func extractField(data []byte, key string) string {
	regex := regexp.MustCompile(fmt.Sprintf(`(?m)^%s=([^\s]+)`, key))
	matches := regex.FindStringSubmatch(string(data))
	if len(matches) == 2 {
		return matches[1]
	}

	return ""
}
