package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

var ErrConfigNotFound = errors.New("config not found")
var ErrFailedToConfigure = errors.New("failed to configure")

var staticConfigs = map[string]map[osinfo.Distribution]map[string]string{
	NginxPackage: {
		Default: {
			"nginx_conf":       "/etc/nginx/nginx.conf",
			"gameap_host_conf": "/etc/nginx/conf.d/gameap.conf",
		},
		DistributionDebian: {
			"nginx_conf":       "/etc/nginx/nginx.conf",
			"gameap_host_conf": "/etc/nginx/conf.d/gameap.conf",
		},
		DistributionUbuntu: {
			"nginx_conf":       "/etc/nginx/nginx.conf",
			"gameap_host_conf": "/etc/nginx/conf.d/gameap.conf",
		},
		DistributionWindows: {
			"nginx_conf":       "C:\\gameap\\tools\\nginx\\conf\\nginx.conf",
			"gameap_host_conf": "C:\\gameap\\tools\\nginx\\conf\\gameap.conf",
		},
	},
	ApachePackage: {
		Default: {
			"gameap_host_conf": "/etc/apache2/sites-available/gameap.conf",
		},
		DistributionDebian: {
			"gameap_host_conf": "/etc/apache2/sites-available/gameap.conf",
		},
		DistributionUbuntu: {
			"gameap_host_conf": "/etc/apache2/sites-available/gameap.conf",
		},
		DistributionWindows: {
			"gameap_host_conf": "C:\\gameap\\tools\\apache2\\sites-available\\gameap.conf",
		},
	},
	PHPPackage: {
		DistributionWindows: {
			"fpm_sock": "127.0.0.1:9934",
		},
	},
}

//nolint:goconst
var dynamicConfig = map[string]map[osinfo.Distribution]map[string]func(ctx context.Context) (string, error){
	PHPPackage: {
		DistributionDefault: {
			"fpm_sock": func(_ context.Context) (string, error) {
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
		DistributionCentOS: {
			"fpm_sock": func(ctx context.Context) (string, error) {
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
			"fpm_sock": func(_ context.Context) (string, error) {
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
			"fpm_sock": func(_ context.Context) (string, error) {
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
			"nginx_conf": func(ctx context.Context) (string, error) {
				path, err := defineNginxPath(ctx)
				if err != nil {
					return "", errors.WithMessage(err, "failed to define nginx path")
				}

				return filepath.Join(path, "conf", "nginx.conf"), nil
			},
			"gameap_host_conf": func(ctx context.Context) (string, error) {
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
		path, err = defineWindowsServiceBinaryPath(ctx, NginxPackage)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to define service binary path"))

			return "", NewErrNotFound(errors.WithMessage(err, "failed to define service binary path").Error())
		}

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

func defineWindowsServiceBinaryPath(_ context.Context, serviceName string) (string, error) {
	cmd := exec.Command("sc", "qc", serviceName)
	buf := &bytes.Buffer{}
	buf.Grow(1024) //nolint:mnd
	cmd.Stdout = buf
	cmd.Stderr = log.Writer()

	log.Println("\n", cmd.String())

	err := cmd.Run()
	if err != nil {
		return "", errors.WithMessage(err, "failed to execute command")
	}

	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		//nolint:gocritic
		switch key {
		case "BINARY_PATH_NAME":
			return value, nil
		}
	}

	return "", nil
}
