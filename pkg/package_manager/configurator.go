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
	"github.com/pkg/errors"
)

var ErrConfigNotFound = errors.New("config not found")

var staticConfigs = map[string]map[string]map[string]string{
	NginxPackage: {
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

var dynamicConfig = map[string]map[string]map[string]func(ctx context.Context) (string, error){
	PHPPackage: {
		DistributionDebian: {
			"fpm_sock": func(_ context.Context) (string, error) {
				if _, err := os.Stat(filepath.Join(chrootPHPPath, packageMarkFile)); err == nil {
					return fmt.Sprintf("unix:%s/php-fpm.sock", chrootPHPPath), nil
				}

				phpVerion, err := DefinePHPVersion()
				if err != nil {
					return "", err
				}

				return fmt.Sprintf("unix:/var/run/php/php%s-fpm.sock", phpVerion), nil
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

				return fmt.Sprintf("unix:/var/run/php/php%s-fpm.sock", phpVerion), nil
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

	// Dynamic config
	//nolint:nestif
	if _, ok := dynamicConfig[packName]; ok {
		if _, ok = dynamicConfig[packName][osInfo.Distribution]; ok {
			if _, ok = dynamicConfig[packName][osInfo.Distribution][configName]; ok {
				config, err := dynamicConfig[packName][osInfo.Distribution][configName](ctx)
				if err != nil {
					return "", errors.WithMessage(err, "failed to get dynamic config")
				}
				if config != "" {
					return config, nil
				}
			}
		}
	}

	// Static config
	if _, ok := staticConfigs[packName]; !ok {
		return "", ErrConfigNotFound
	}

	if _, ok := staticConfigs[packName][osInfo.Distribution]; !ok {
		return "", ErrConfigNotFound
	}

	if _, ok := staticConfigs[packName][osInfo.Distribution][configName]; !ok {
		return "", ErrConfigNotFound
	}

	return staticConfigs[packName][osInfo.Distribution][configName], nil
}

func defineNginxPath(ctx context.Context) (string, error) {
	path, err := findNginxDirWindows(ctx)
	if err != nil {
		return "", errors.WithMessage(err, "failed to find nginx directory")
	}

	if path == "" {
		path, err = defineWindowsServiceBinaryPath(ctx, NginxPackage)
		if err != nil {
			return "", errors.WithMessage(err, "failed to define service binary path")
		}

		stat, err := os.Stat(path)
		if err != nil {
			return "", err
		}

		if !stat.IsDir() {
			path = filepath.Dir(path)
		}
	}

	if path == "" {
		return "", errors.New("nginx binary not found")
	}

	if _, err := os.Stat(filepath.Join(path, "nginx.exe")); err == nil {
		return path, nil
	}

	return "", errors.New("nginx path not found")
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
	buf.Grow(1024) //nolint:gomnd
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
