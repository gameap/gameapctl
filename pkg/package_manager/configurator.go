package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"log"
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
		DistributionDebian: {
			"fpm_sock": "unix:/var/run/php/php%s-fpm.sock",
		},
		DistributionUbuntu: {
			"fpm_sock": "unix:/var/run/php/php%s-fpm.sock",
		},
		DistributionWindows: {
			"fpm_sock": "127.0.0.1:9934",
		},
	},
}

var dynamicConfig = map[string]map[string]map[string]func(ctx context.Context) (string, error){
	NginxPackage: {
		DistributionWindows: {
			"nginx_conf": func(ctx context.Context) (string, error) {
				nginxBinaryPath, err := defineWindowsServiceBinaryPath(ctx, NginxPackage)
				if err != nil {
					return "", err
				}

				return filepath.Join(nginxBinaryPath, "conf\\nginx.conf"), nil
			},
			"gameap_host_conf": func(ctx context.Context) (string, error) {
				nginxBinaryPath, err := defineWindowsServiceBinaryPath(ctx, NginxPackage)
				if err != nil {
					return "", err
				}

				return filepath.Join(nginxBinaryPath, "conf\\gameap.conf"), nil
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

func defineWindowsServiceBinaryPath(_ context.Context, serviceName string) (string, error) {
	cmd := exec.Command("sc", "qc", serviceName)
	buf := &bytes.Buffer{}
	buf.Grow(1024)
	cmd.Stdout = buf
	cmd.Stderr = log.Writer()

	log.Println("\n", cmd.String())

	err := cmd.Run()
	if err != nil {
		return "", err
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
