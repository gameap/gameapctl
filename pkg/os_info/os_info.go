package osinfo

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/matishsiao/goInfo"
	"github.com/pkg/errors"
)

type Distribution string

const (
	DistributionAlmaLinux Distribution = "almalinux"
	DistributionAlpine    Distribution = "alpine"
	DistributionAmazon    Distribution = "amzn"
	DistributionArch      Distribution = "arch"
	DistributionCentOS    Distribution = "centos"
	DistributionDebian    Distribution = "debian"
	DistributionFedora    Distribution = "fedora"
	DistributionGentoo    Distribution = "gentoo"
	DistributionOpenSUSE  Distribution = "opensuse"
	DistributionRaspbian  Distribution = "raspbian"
	DistributionRocky     Distribution = "rocky"
	DistributionUbuntu    Distribution = "ubuntu"
	DistributionUnknown   Distribution = "unknown"

	DistributionWindows Distribution = "windows"
)

func (d Distribution) IsDebianLike() bool {
	return d == DistributionDebian || d == DistributionUbuntu || d == DistributionRaspbian
}

func (d Distribution) IsWindows() bool {
	return d == DistributionWindows
}

type Platform string

const (
	PlatformX86    Platform = "x86"
	PlatformAmd64  Platform = "amd64"
	PlatformArm    Platform = "arm"
	PlatformArm64  Platform = "arm64"
	PlatformMips   Platform = "mips"
	PlatformMips64 Platform = "mips64"

	PlatformUnknown Platform = "unknown"
)

func (p Platform) IsX86() bool {
	return p == PlatformX86 || p == PlatformAmd64
}

func (p Platform) String() string {
	return string(p)
}

type Info struct {
	Kernel               string
	Core                 string
	Distribution         Distribution
	DistributionVersion  string
	DistributionCodename string
	Platform             Platform
	OS                   string
	Hostname             string
	CPUs                 int
}

func (info Info) String() string {
	b := strings.Builder{}
	b.Grow(256) //nolint:mnd

	b.WriteString("Kernel: ")
	b.WriteString(info.Kernel)
	b.WriteString("\nCore: ")
	b.WriteString(info.Core)
	b.WriteString("\nDistribution: ")
	b.WriteString(string(info.Distribution))
	b.WriteString("\nDistributionVersion: ")
	b.WriteString(info.DistributionVersion)
	b.WriteString("\nDistributionCodename: ")
	b.WriteString(info.DistributionCodename)
	b.WriteString("\nPlatform: ")
	b.WriteString(info.Platform.String())
	b.WriteString("\nOS: ")
	b.WriteString(info.OS)
	b.WriteString("\nHostname: ")
	b.WriteString(info.Hostname)
	b.WriteString("\nCPUs: ")
	b.WriteString(strconv.Itoa(info.CPUs))

	return b.String()
}

func (info Info) IsDebianLike() bool {
	return info.Distribution.IsDebianLike()
}

func (info Info) IsWindows() bool {
	return info.Distribution.IsWindows()
}

func (info Info) IsLinux() bool {
	return runtime.GOOS == "linux" || info.OS == "GNU/Linux"
}

func GetOSInfo() (Info, error) {
	gi, err := goInfo.GetInfo()
	if err != nil {
		return Info{}, err
	}

	result := Info{
		Kernel:   gi.Kernel,
		Core:     gi.Core,
		OS:       gi.OS,
		Hostname: gi.Hostname,
		CPUs:     gi.CPUs,
	}

	result.Platform = detectPlatform()

	if gi.OS == "GNU/Linux" {
		info, err := detectLinuxDist()
		if err != nil {
			return result, err
		}
		result.Distribution = Distribution(info.Name)
		result.DistributionVersion = info.Version
		result.DistributionCodename = info.VersionCodename
	} else {
		result.Distribution = Distribution(gi.OS)
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

//nolint:gocognit,funlen
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
			return distInfo{}, err
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
			return distInfo{}, err
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
			return distInfo{}, err
		}
		result.VersionCodename = strings.Split(string(out), ":")[1]
		result.VersionCodename = strings.TrimSpace(result.VersionCodename)

		// extract os from lsb_release -i
		cmd = exec.Command("lsb_release", "-i")
		cmd.Stderr = os.Stderr
		out, err = cmd.Output()
		if err != nil {
			return distInfo{}, err
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
			return distInfo{}, err
		}
		result.Name = strings.Split(string(data), " ")[0]
		result.Name = strings.TrimSpace(result.Name)
		result.Name = strings.ToLower(result.Name)

		// extract dist from /etc/debian_version
		data, err = os.ReadFile("/etc/debian_version")
		if err != nil {
			return distInfo{}, err
		}
		result.VersionCodename = strings.Split(string(data), "/")[0]
		result.VersionCodename = strings.TrimSpace(result.VersionCodename)
	}

	if result.VersionCodename == "" {
		// unknown os
		return distInfo{}, errors.New("unknown operating system")
	}

	// cleanup
	result.Name = strings.ReplaceAll(result.Name, " ", "")
	result.VersionCodename = strings.ReplaceAll(result.VersionCodename, " ", "")
	result.Name = strings.Trim(result.Name, "\"")
	result.VersionCodename = strings.Trim(result.VersionCodename, "\"")

	// lowercase
	result.Name = strings.ToLower(result.Name)
	result.VersionCodename = strings.ToLower(result.VersionCodename)

	return result, nil
}

func detectPlatform() Platform {
	switch runtime.GOARCH {
	case "386":
		return PlatformX86
	case "amd64":
		return PlatformAmd64
	case "arm":
		return PlatformArm
	case "arm64":
		return PlatformArm64
	case "mips":
		return PlatformMips
	case "mips64":
		return PlatformMips64
	}

	return PlatformUnknown
}

func extractField(data []byte, key string) string {
	regex := regexp.MustCompile(fmt.Sprintf(`(?m)^%s=([^\s]+)`, key))
	matches := regex.FindStringSubmatch(string(data))
	if len(matches) == 2 {
		return matches[1]
	}

	return ""
}
