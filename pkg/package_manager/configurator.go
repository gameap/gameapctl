package packagemanager

import (
	"context"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/pkg/errors"
)

var ErrConfigNotFound = errors.New("config not found")

var configs = map[string]map[string]map[string]string{
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
			"fpm_sock": "",
		},
	},
}

func ConfigForDistro(ctx context.Context, packName string, configName string) (string, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	if _, ok := configs[packName]; !ok {
		return "", ErrConfigNotFound
	}

	if _, ok := configs[packName][osInfo.Distribution]; !ok {
		return "", ErrConfigNotFound
	}

	if _, ok := configs[packName][osInfo.Distribution][configName]; !ok {
		return "", ErrConfigNotFound
	}

	return configs[packName][osInfo.Distribution][configName], nil
}
