package packagemanager

import (
	"context"
	"testing"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	pmdnf "github.com/gameap/gameapctl/pkg/package_manager/dnf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseYumInfoOutput(t *testing.T) {
	out := []byte(`
Last metadata expiration check: 0:09:02 ago on Fri 17 Nov 2023 04:11:17 AM UTC.
Available Packages
Name         : mysql
Version      : 8.0.32
Release      : 1.el9
Architecture : x86_64
Size         : 2.8 M
Source       : mysql-8.0.32-1.el9.src.rpm
Repository   : appstream
Summary      : MySQL client programs and shared libraries
URL          : http://www.mysql.com
License      : GPLv2 with exceptions and LGPLv2 and BSD
Description  : MySQL is a multi-user, multi-threaded SQL database server. MySQL is a
             : client/server implementation consisting of a server daemon (mysqld)
             : and many different client programs and libraries. The base package
             : contains the standard MySQL client programs and generic MySQL files.

Name         : mysql-common
Version      : 8.0.32
Release      : 1.el9
Architecture : x86_64
Size         : 75 k
Source       : mysql-8.0.32-1.el9.src.rpm
Repository   : appstream
Summary      : The shared files required for MySQL server and client
URL          : http://www.mysql.com
License      : GPLv2 with exceptions and LGPLv2 and BSD
Description  : The mysql-common package provides the essential shared files for any
             : MySQL program. You will need to install this package to use any other
             : MySQL package.

	`)
	parsed, err := parseYumInfoOutput(out)

	require.NoError(t, err)
	require.Equal(t, []PackageInfo{
		{
			Name:         "mysql",
			Version:      "8.0.32",
			Architecture: "x86_64",
			Size:         "2.8 M",
			Description: "MySQL is a multi-user, multi-threaded SQL database server. " +
				"MySQL is a client/server implementation consisting of a server daemon (mysqld)" +
				" and many different client programs and libraries. The base package contains the " +
				"standard MySQL client programs and generic MySQL files.",
		},
		{
			Name:         "mysql-common",
			Version:      "8.0.32",
			Architecture: "x86_64",
			Size:         "75 k",
			Description: "The mysql-common package provides the essential shared files for any " +
				"MySQL program. You will need to install this package to use any other MySQL package.",
		},
	}, parsed)
}

func Test_extendedDNF_preInstallationSteps(t *testing.T) {
	t.Run("no pre-install steps", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"test-package": {
					Name:        "test-package",
					ReplaceWith: []string{"actual-package"},
				},
			},
		}

		packs, err := d.preInstallationSteps(context.Background(), "test-package")
		require.NoError(t, err)
		assert.Equal(t, []string{"test-package"}, packs)
	})

	t.Run("package not in config", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{},
		}

		packs, err := d.preInstallationSteps(context.Background(), "unknown-package")
		require.NoError(t, err)
		assert.Equal(t, []string{"unknown-package"}, packs)
	})

	t.Run("multiple packages with different configs", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"package-with-pre": {
					Name:       "package-with-pre",
					PreInstall: []string{"echo pre-install"},
				},
				"package-without-pre": {
					Name: "package-without-pre",
				},
			},
		}

		packs, err := d.preInstallationSteps(context.Background(), "package-with-pre", "package-without-pre")
		require.NoError(t, err)
		assert.Equal(t, []string{"package-with-pre", "package-without-pre"}, packs)
	})

	t.Run("duplicate packages in list", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"test-package": {
					Name:       "test-package",
					PreInstall: []string{"echo test"},
				},
			},
		}

		packs, err := d.preInstallationSteps(context.Background(), "test-package", "test-package")
		require.NoError(t, err)
		assert.Equal(t, []string{"test-package", "test-package"}, packs)
	})
}

func Test_extendedDNF_postInstallationSteps(t *testing.T) {
	t.Run("no post-install steps", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"test-package": {
					Name:        "test-package",
					ReplaceWith: []string{"actual-package"},
				},
			},
		}

		err := d.postInstallationSteps(context.Background(), "test-package")
		require.NoError(t, err)
	})

	t.Run("package not in config", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{},
		}

		err := d.postInstallationSteps(context.Background(), "unknown-package")
		require.NoError(t, err)
	})

	t.Run("multiple packages with different configs", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"package-with-post": {
					Name:        "package-with-post",
					PostInstall: []string{"echo post-install"},
				},
				"package-without-post": {
					Name: "package-without-post",
				},
			},
		}

		err := d.postInstallationSteps(context.Background(), "package-with-post", "package-without-post")
		require.NoError(t, err)
	})

	t.Run("duplicate packages in list", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"test-package": {
					Name:        "test-package",
					PostInstall: []string{"echo test"},
				},
			},
		}

		err := d.postInstallationSteps(context.Background(), "test-package", "test-package")
		require.NoError(t, err)
	})
}

func Test_extendedDNF_executeCommand(t *testing.T) {
	d := &extendedDNF{}

	t.Run("empty command", func(t *testing.T) {
		err := d.executeCommand(context.Background(), "")
		require.NoError(t, err)
	})

	t.Run("whitespace only command", func(t *testing.T) {
		err := d.executeCommand(context.Background(), "   ")
		require.NoError(t, err)
	})

	t.Run("simple command", func(t *testing.T) {
		err := d.executeCommand(context.Background(), "echo test")
		require.NoError(t, err)
	})

	t.Run("command with multiple arguments", func(t *testing.T) {
		err := d.executeCommand(context.Background(), "echo hello world")
		require.NoError(t, err)
	})
}

func Test_extendedDNF_replaceAliases(t *testing.T) {
	t.Run("replace with configured packages", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"php": {
					Name:        "php",
					ReplaceWith: []string{"php-cli", "php-common", "php-fpm"},
				},
			},
		}

		result := d.replaceAliases(context.Background(), []string{"php"})
		assert.Equal(t, []string{"php-cli", "php-common", "php-fpm"}, result)
	})

	t.Run("keep unknown packages as is", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{},
		}

		result := d.replaceAliases(context.Background(), []string{"unknown-package"})
		assert.Equal(t, []string{"unknown-package"}, result)
	})

	t.Run("mixed known and unknown packages", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"php": {
					Name:        "php",
					ReplaceWith: []string{"php-cli", "php-fpm"},
				},
			},
		}

		result := d.replaceAliases(context.Background(), []string{"php", "nginx", "mysql"})
		assert.Equal(t, []string{"php-cli", "php-fpm", "nginx", "mysql"}, result)
	})

	t.Run("empty replace-with array", func(t *testing.T) {
		d := &extendedDNF{
			packages: map[string]pmdnf.PackageConfig{
				"lib32gcc": {
					Name:        "lib32gcc",
					ReplaceWith: []string{},
				},
			},
		}

		result := d.replaceAliases(context.Background(), []string{"lib32gcc"})
		assert.Empty(t, result)
	})
}

func Test_newExtendedDNF(t *testing.T) {
	t.Run("creates extended dnf with packages", func(t *testing.T) {
		osInfo := osinfo.Info{
			Distribution:        osinfo.DistributionCentOS,
			DistributionVersion: "8",
			Platform:            osinfo.PlatformAmd64,
		}

		mockDNF := &dnf{}
		extended, err := newExtendedDNF(osInfo, mockDNF)

		require.NoError(t, err)
		require.NotNil(t, extended)
		assert.NotNil(t, extended.packages)
		assert.NotNil(t, extended.underlined)
	})
}
