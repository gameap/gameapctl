package packagemanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

var ErrConfigNotFound = errors.New("config not found")
var ErrFailedToConfigure = errors.New("failed to configure")

const (
	ConfigurationNginxConf      = "nginx_conf"
	ConfigurationGameAPHostConf = "gameap_host_conf"
	ConfigurationPHPFpmSock     = "fpm_sock"
)

var staticConfigs = map[string]map[osinfo.Distribution]map[string]string{
	NginxPackage: {
		Default: {
			ConfigurationNginxConf:      "/etc/nginx/nginx.conf",
			ConfigurationGameAPHostConf: "/etc/nginx/conf.d/gameap.conf",
		},
		DistributionDebian: {
			ConfigurationNginxConf:      "/etc/nginx/nginx.conf",
			ConfigurationGameAPHostConf: "/etc/nginx/conf.d/gameap.conf",
		},
		DistributionUbuntu: {
			ConfigurationNginxConf:      "/etc/nginx/nginx.conf",
			ConfigurationGameAPHostConf: "/etc/nginx/conf.d/gameap.conf",
		},
		DistributionWindows: {
			ConfigurationNginxConf:      "C:\\gameap\\tools\\nginx\\conf\\nginx.conf",
			ConfigurationGameAPHostConf: "C:\\gameap\\tools\\nginx\\conf\\gameap.conf",
		},
	},
	ApachePackage: {
		Default: {
			ConfigurationGameAPHostConf: "/etc/apache2/sites-available/gameap.conf",
		},
		DistributionDebian: {
			ConfigurationGameAPHostConf: "/etc/apache2/sites-available/gameap.conf",
		},
		DistributionUbuntu: {
			ConfigurationGameAPHostConf: "/etc/apache2/sites-available/gameap.conf",
		},
		DistributionWindows: {
			ConfigurationGameAPHostConf: "C:\\gameap\\tools\\apache2\\sites-available\\gameap.conf",
		},
	},
	PHPPackage: {
		DistributionWindows: {
			ConfigurationPHPFpmSock: "127.0.0.1:9934",
		},
	},
}

var dynamicConfig = map[string]map[osinfo.Distribution]map[string]func(ctx context.Context) (string, error){
	PHPPackage: {
		DistributionDefault: {
			ConfigurationPHPFpmSock: func(_ context.Context) (string, error) {
				if _, err := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); err == nil {
					return fmt.Sprintf("unix:%s/php-fpm.sock", chrootPHPPath), nil
				}

				phpVerion, err := DefinePHPVersion()
				if err != nil {
					return "", err
				}

				switch {
				case utils.IsFileExists(fmt.Sprintf("/var/run/php/php%s-fpm.sock", phpVerion)):
					return fmt.Sprintf("unix:/var/run/php/php%s-fpm.sock", phpVerion), nil
				case utils.IsFileExists(fmt.Sprintf("/run/php/php%s-fpm.sock", phpVerion)):
					return fmt.Sprintf("unix:/run/php/php%s-fpm.sock", phpVerion), nil
				case utils.IsFileExists("/etc/alternatives/php-fpm.sock"):
					return "unix:/etc/alternatives/php-fpm.sock", nil //nolint:goconst
				case utils.IsFileExists("/var/run/php-fpm/www.sock"):
					return "unix:/var/run/php-fpm/www.sock", nil //nolint:goconst
				case utils.IsFileExists("/var/run/php/php-fpm.sock"):
					return "unix:/var/run/php/php-fpm.sock", nil //nolint:goconst
				}

				return "", ErrFailedToConfigure
			},
		},
		DistributionCentOS: {
			ConfigurationPHPFpmSock: func(ctx context.Context) (string, error) {
				if utils.IsFileExists(filepath.Join(chrootPHPPath, packageMarkFile)) {
					return fmt.Sprintf("unix:%s/php-fpm.sock", chrootPHPPath), nil
				}

				osInfo := contextInternal.OSInfoFromContext(ctx)
				if osInfo.DistributionCodename == "7" {
					return "127.0.0.1:9000", nil
				}

				switch {
				case utils.IsFileExists("/etc/alternatives/php-fpm.sock"):
					return "unix:/etc/alternatives/php-fpm.sock", nil
				case utils.IsFileExists("/var/run/php-fpm/www.sock"):
					return "unix:/var/run/php-fpm/www.sock", nil
				case utils.IsFileExists("/var/run/php/php-fpm.sock"):
					return "unix:/var/run/php/php-fpm.sock", nil
				}

				return "", ErrFailedToConfigure
			},
		},
		DistributionDebian: {
			ConfigurationPHPFpmSock: func(_ context.Context) (string, error) {
				if _, err := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); err == nil {
					return fmt.Sprintf("unix:%s/php-fpm.sock", chrootPHPPath), nil
				}

				phpVerion, err := DefinePHPVersion()
				if err != nil {
					return "", err
				}

				switch {
				case utils.IsFileExists(fmt.Sprintf("/var/run/php/php%s-fpm.sock", phpVerion)):
					return fmt.Sprintf("unix:/var/run/php/php%s-fpm.sock", phpVerion), nil
				case utils.IsFileExists(fmt.Sprintf("/run/php/php%s-fpm.sock", phpVerion)):
					return fmt.Sprintf("unix:/run/php/php%s-fpm.sock", phpVerion), nil
				case utils.IsFileExists("/etc/alternatives/php-fpm.sock"):
					return "unix:/etc/alternatives/php-fpm.sock", nil
				case utils.IsFileExists("/var/run/php-fpm/www.sock"):
					return "unix:/var/run/php-fpm/www.sock", nil
				case utils.IsFileExists("/var/run/php/php-fpm.sock"):
					return "unix:/var/run/php/php-fpm.sock", nil
				}

				return "", ErrFailedToConfigure
			},
		},
		DistributionUbuntu: {
			ConfigurationPHPFpmSock: func(_ context.Context) (string, error) {
				if _, err := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); err == nil {
					return fmt.Sprintf("unix:%s/php-fpm.sock", chrootPHPPath), nil
				}

				phpVerion, err := DefinePHPVersion()
				if err != nil {
					return "", err
				}

				switch {
				case utils.IsFileExists(fmt.Sprintf("/var/run/php/php%s-fpm.sock", phpVerion)):
					return fmt.Sprintf("unix:/var/run/php/php%s-fpm.sock", phpVerion), nil
				case utils.IsFileExists(fmt.Sprintf("/run/php/php%s-fpm.sock", phpVerion)):
					return fmt.Sprintf("unix:/run/php/php%s-fpm.sock", phpVerion), nil
				case utils.IsFileExists("/etc/alternatives/php-fpm.sock"):
					return "unix:/etc/alternatives/php-fpm.sock", nil
				case utils.IsFileExists("/var/run/php-fpm/www.sock"):
					return "unix:/var/run/php-fpm/www.sock", nil
				case utils.IsFileExists("/var/run/php/php-fpm.sock"):
					return "unix:/var/run/php/php-fpm.sock", nil
				}

				return "", ErrFailedToConfigure
			},
		},
	},
	NginxPackage: {
		DistributionWindows: {
			ConfigurationNginxConf: func(ctx context.Context) (string, error) {
				path, err := defineNginxPath(ctx)
				if err != nil {
					return "", errors.WithMessage(err, "failed to define nginx path")
				}

				return filepath.Join(path, "conf", "nginx.conf"), nil
			},
			ConfigurationGameAPHostConf: func(ctx context.Context) (string, error) {
				path, err := defineNginxPath(ctx)
				if err != nil {
					return "", errors.WithMessage(err, "failed to define nginx path")
				}

				return filepath.Join(path, "conf", "gameap.conf"), nil
			},
		},
	},
}

func ConfigForDistro(ctx context.Context, packName string, configName string) (string, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	// Static config
	if _, ok := staticConfigs[packName][osInfo.Distribution][configName]; ok {
		return staticConfigs[packName][osInfo.Distribution][configName], nil
	}

	if _, ok := staticConfigs[packName][Default][configName]; ok {
		return staticConfigs[packName][Default][configName], nil
	}

	// Dynamic config
	if configFunc, ok := dynamicConfig[packName][osInfo.Distribution][configName]; ok {
		config, err := configFunc(ctx)
		if err != nil {
			return "", errors.WithMessage(err, "failed to get dynamic config")
		}
		if config != "" {
			return config, nil
		}
	}
	if configFunc, ok := dynamicConfig[packName][DistributionDefault][configName]; ok {
		config, err := configFunc(ctx)
		if err != nil {
			return "", errors.WithMessage(err, "failed to get dynamic config")
		}
		if config != "" {
			return config, nil
		}
	}

	return "", ErrConfigNotFound
}

func defineNginxPath(ctx context.Context) (string, error) {
	path, err := findNginxDirWindows(ctx)
	if err != nil {
		return "", NewErrNotFound("failed to find nginx directory")
	}

	if path == "" {
		binaryPath, err := windowsServiceBinaryPath(NginxPackage)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to define service binary path"))

			return "", NewErrNotFound(errors.WithMessage(err, "failed to define service binary path").Error())
		}

		path = windowsCommandExecutable(binaryPath)

		stat, err := os.Stat(path)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to stat nginx binary"))

			return "", NewErrNotFound(errors.WithMessage(err, "failed to stat nginx binary").Error())
		}

		if !stat.IsDir() {
			path = filepath.Dir(path)
		}
	}

	if path == "" {
		return "", NewErrNotFound("nginx binary not found")
	}

	if _, err := os.Stat(filepath.Join(path, "nginx.exe")); err == nil {
		return path, nil
	}

	return "", NewErrNotFound("nginx path not found")
}

func findNginxDirWindows(_ context.Context) (string, error) {
	directories := []string{
		"C:\\tools",
		"C:\\gameap\\tools",
	}

	for _, dir := range directories {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}

		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "nginx") {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
	}

	return "", nil
}

// windowsCommandExecutable returns the executable of a Windows command line. A service binary
// path keeps the arguments the service was registered with, and a path containing spaces is
// quoted, so neither can be passed to the file system as is.
func windowsCommandExecutable(commandLine string) string {
	commandLine = strings.TrimSpace(commandLine)

	if rest, quoted := strings.CutPrefix(commandLine, `"`); quoted {
		executable, _, _ := strings.Cut(rest, `"`)

		return executable
	}

	executable, _, _ := strings.Cut(commandLine, " ")

	return executable
}
