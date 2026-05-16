package pkgconfig

import (
	"testing"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPackages_APT_Default(t *testing.T) {
	packages, err := LoadPackages("apt", osinfo.Info{})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("postgresql package", func(t *testing.T) {
		pkg, exists := packages["postgresql"]
		require.True(t, exists, "postgresql should exist in default.yaml")
		assert.Equal(t, "postgresql", pkg.Name)
		assert.Equal(t, []string{"postgresql", "postgresql-contrib"}, pkg.ReplaceWith)
		assert.Empty(t, pkg.PreInstall)
		require.Len(t, pkg.PostInstall, 2)
		assert.Empty(t, pkg.Install)
	})

	t.Run("lib32gcc package", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in default.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Empty(t, pkg.ReplaceWith)
	})

	t.Run("composer package with install", func(t *testing.T) {
		pkg, exists := packages["composer"]
		require.True(t, exists, "composer should exist in default.yaml")
		assert.Equal(t, "composer", pkg.Name)
		require.Len(t, pkg.Install, 1)
		require.Len(t, pkg.Install[0].RunCommands, 1)
		assert.Contains(t, pkg.Install[0].RunCommands[0], "curl -sS https://getcomposer.org/installer")
		assert.Contains(t, pkg.Install[0].RunCommands[0], "php -- --install-dir=/usr/local/bin --filename=composer")
	})

	t.Run("nodejs with pre-install", func(t *testing.T) {
		pkg, exists := packages["nodejs"]
		require.True(t, exists, "nodejs should exist in default.yaml")
		assert.Equal(t, "nodejs", pkg.Name)
		require.Len(t, pkg.PreInstall, 2)
		require.Len(t, pkg.PreInstall[0].RunCommands, 2)
		require.Len(t, pkg.PreInstall[1].RunCommands, 2)
		assert.Equal(t, "mkdir -p /usr/share/keyrings/", pkg.PreInstall[0].RunCommands[0])
		assert.Contains(t, pkg.PreInstall[0].RunCommands[1], "curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key")
		assert.Contains(t, pkg.PreInstall[0].RunCommands[1], "gpg --dearmor -o /usr/share/keyrings/nodesource.gpg")
		assert.Contains(t, pkg.PreInstall[1].RunCommands[0], "deb [signed-by=/usr/share/keyrings/nodesource.gpg]")
		assert.Contains(t, pkg.PreInstall[1].RunCommands[0], "https://deb.nodesource.com/node_24.x")
	})
}

func TestLoadPackages_APT_ARM64(t *testing.T) {
	packages, err := LoadPackages("apt", osinfo.Info{
		Platform: osinfo.PlatformArm64,
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("lib32gcc with arm64 replacement", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in default_arm64.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Equal(t, []string{"lib32gcc-s1-amd64-cross"}, pkg.ReplaceWith)
	})

	t.Run("lib32stdc++6 with arm64 replacement", func(t *testing.T) {
		pkg, exists := packages["lib32stdc++6"]
		require.True(t, exists, "lib32stdc++6 should exist in default_arm64.yaml")
		assert.Equal(t, "lib32stdc++6", pkg.Name)
		assert.Equal(t, []string{"lib32stdc++6-amd64-cross"}, pkg.ReplaceWith)
	})

	t.Run("lib32z1 with empty replacement", func(t *testing.T) {
		pkg, exists := packages["lib32z1"]
		require.True(t, exists, "lib32z1 should exist in default_arm64.yaml")
		assert.Equal(t, "lib32z1", pkg.Name)
		assert.Empty(t, pkg.ReplaceWith)
	})
}

func TestLoadPackages_APT_Debian12(t *testing.T) {
	packages, err := LoadPackages("apt", osinfo.Info{
		Distribution:        osinfo.DistributionDebian,
		DistributionVersion: "12",
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("mysql-server replaced with default-mysql-server", func(t *testing.T) {
		pkg, exists := packages["mysql-server"]
		require.True(t, exists, "mysql-server should exist in debian_12.yaml")
		assert.Equal(t, "mysql-server", pkg.Name)
		assert.Equal(t, []string{"default-mysql-server"}, pkg.ReplaceWith)
	})

	t.Run("lib32gcc replaced with lib32gcc-s1", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in debian_12.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Equal(t, []string{"lib32gcc-s1"}, pkg.ReplaceWith)
	})

	t.Run("nodejs replaced with nodejs and npm", func(t *testing.T) {
		pkg, exists := packages["nodejs"]
		require.True(t, exists, "nodejs should exist in debian_12.yaml")
		assert.Equal(t, "nodejs", pkg.Name)
		assert.Equal(t, []string{"nodejs", "npm"}, pkg.ReplaceWith)
	})
}

func TestLoadPackages_APT_DebianSid(t *testing.T) {
	packages, err := LoadPackages("apt", osinfo.Info{
		Distribution:         osinfo.DistributionDebian,
		DistributionCodename: "sid",
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("mysql-server replaced with default-mysql-server", func(t *testing.T) {
		pkg, exists := packages["mysql-server"]
		require.True(t, exists, "mysql-server should exist in debian_sid.yaml")
		assert.Equal(t, "mysql-server", pkg.Name)
		assert.Equal(t, []string{"default-mysql-server"}, pkg.ReplaceWith)
	})

	t.Run("lib32gcc replaced with lib32gcc-s1", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in debian_sid.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Equal(t, []string{"lib32gcc-s1"}, pkg.ReplaceWith)
	})
}

func TestLoadPackages_APT_FileHierarchy(t *testing.T) {
	t.Run("default overridden by arch-specific", func(t *testing.T) {
		packages, err := LoadPackages("apt", osinfo.Info{
			Platform: osinfo.PlatformArm64,
		})
		require.NoError(t, err)

		pkg, exists := packages["lib32gcc"]
		require.True(t, exists)
		assert.Equal(t, []string{"lib32gcc-s1-amd64-cross"}, pkg.ReplaceWith)
	})

	t.Run("default overridden by distribution version", func(t *testing.T) {
		packages, err := LoadPackages("apt", osinfo.Info{
			Distribution:        osinfo.DistributionDebian,
			DistributionVersion: "12",
		})
		require.NoError(t, err)

		pkg, exists := packages["lib32gcc"]
		require.True(t, exists)
		assert.Equal(t, []string{"lib32gcc-s1"}, pkg.ReplaceWith)
	})

	t.Run("default overridden by distribution codename", func(t *testing.T) {
		packages, err := LoadPackages("apt", osinfo.Info{
			Distribution:         osinfo.DistributionDebian,
			DistributionCodename: "sid",
		})
		require.NoError(t, err)

		pkg, exists := packages["lib32gcc"]
		require.True(t, exists)
		assert.Equal(t, []string{"lib32gcc-s1"}, pkg.ReplaceWith)
	})
}

func TestLoadPackages_DNF_Default(t *testing.T) {
	packages, err := LoadPackages("dnf", osinfo.Info{})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("lib32gcc package", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in default.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Equal(t, []string{"libgcc.i686"}, pkg.ReplaceWith)
		assert.Empty(t, pkg.PreInstall)
		assert.Empty(t, pkg.PostInstall)
	})

	t.Run("lib32stdc6 package", func(t *testing.T) {
		pkg, exists := packages["lib32stdc6"]
		require.True(t, exists, "lib32stdc6 should exist in default.yaml")
		assert.Equal(t, "lib32stdc6", pkg.Name)
		assert.Equal(t, []string{"libstdc++.i686"}, pkg.ReplaceWith)
	})

	t.Run("xz-utils package", func(t *testing.T) {
		pkg, exists := packages["xz-utils"]
		require.True(t, exists, "xz-utils should exist in default.yaml")
		assert.Equal(t, "xz-utils", pkg.Name)
		assert.Equal(t, []string{"xz"}, pkg.ReplaceWith)
	})

	t.Run("php-extensions package", func(t *testing.T) {
		pkg, exists := packages["php-extensions"]
		require.True(t, exists, "php-extensions should exist in default.yaml")
		assert.Equal(t, "php-extensions", pkg.Name)
		assert.Contains(t, pkg.ReplaceWith, "php-bcmath")
		assert.Contains(t, pkg.ReplaceWith, "php-gd")
		assert.Contains(t, pkg.ReplaceWith, "php-xml")
	})

	t.Run("postgresql with post-install", func(t *testing.T) {
		pkg, exists := packages["postgresql"]
		require.True(t, exists, "postgresql should exist in default.yaml")
		assert.Equal(t, "postgresql", pkg.Name)
		assert.Equal(t, []string{"postgresql-server", "postgresql-contrib"}, pkg.ReplaceWith)
		require.Len(t, pkg.PostInstall, 2)
		require.Len(t, pkg.PostInstall[0].RunCommands, 3)
		assert.Equal(t, "postgresql-setup --initdb", pkg.PostInstall[0].RunCommands[0])
		assert.Equal(t, "systemctl enable postgresql", pkg.PostInstall[0].RunCommands[1])
		assert.Equal(t, "systemctl start postgresql", pkg.PostInstall[0].RunCommands[2])
	})

	t.Run("redis-server with post-install", func(t *testing.T) {
		pkg, exists := packages["redis-server"]
		require.True(t, exists, "redis-server should exist in default.yaml")
		assert.Equal(t, "redis-server", pkg.Name)
		assert.Equal(t, []string{"redis"}, pkg.ReplaceWith)
		require.Len(t, pkg.PostInstall, 1)
		require.Len(t, pkg.PostInstall[0].RunCommands, 2)
		assert.Equal(t, "systemctl enable redis", pkg.PostInstall[0].RunCommands[0])
		assert.Equal(t, "systemctl start redis", pkg.PostInstall[0].RunCommands[1])
	})
}

func TestLoadPackages_Pacman_Default(t *testing.T) {
	packages, err := LoadPackages("pacman", osinfo.Info{
		Distribution: osinfo.DistributionArch,
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("lib32gcc with multilib pre-install", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in default.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Equal(t, []string{"lib32-gcc-libs"}, pkg.ReplaceWith)
		require.Len(t, pkg.PreInstall, 1)
		require.Len(t, pkg.PreInstall[0].RunCommands, 2)
		assert.Contains(t, pkg.PreInstall[0].RunCommands[0], "[multilib]")
		assert.Contains(t, pkg.PreInstall[0].RunCommands[0], "/etc/pacman.conf")
		assert.Equal(t, "pacman -Sy --noconfirm", pkg.PreInstall[0].RunCommands[1])
	})

	t.Run("lib32stdc6 and lib32stdc++6 map to lib32-gcc-libs", func(t *testing.T) {
		pkg, exists := packages["lib32stdc6"]
		require.True(t, exists, "lib32stdc6 should exist in default.yaml")
		assert.Equal(t, []string{"lib32-gcc-libs"}, pkg.ReplaceWith)

		pkg, exists = packages["lib32stdc++6"]
		require.True(t, exists, "lib32stdc++6 should exist in default.yaml")
		assert.Equal(t, []string{"lib32-gcc-libs"}, pkg.ReplaceWith)
	})

	t.Run("lib32z1 maps to lib32-zlib", func(t *testing.T) {
		pkg, exists := packages["lib32z1"]
		require.True(t, exists, "lib32z1 should exist in default.yaml")
		assert.Equal(t, []string{"lib32-zlib"}, pkg.ReplaceWith)
		require.Len(t, pkg.PreInstall, 1)
	})

	t.Run("xz-utils maps to xz", func(t *testing.T) {
		pkg, exists := packages["xz-utils"]
		require.True(t, exists, "xz-utils should exist in default.yaml")
		assert.Equal(t, []string{"xz"}, pkg.ReplaceWith)
		assert.Empty(t, pkg.PreInstall)
	})

	t.Run("postgresql with initdb post-install", func(t *testing.T) {
		pkg, exists := packages["postgresql"]
		require.True(t, exists, "postgresql should exist in default.yaml")
		assert.Equal(t, []string{"postgresql"}, pkg.ReplaceWith)
		require.Len(t, pkg.PostInstall, 2)
		require.Len(t, pkg.PostInstall[0].RunCommands, 3)
		assert.Contains(t, pkg.PostInstall[0].RunCommands[0], "PG_VERSION")
		assert.Contains(t, pkg.PostInstall[0].RunCommands[0], "initdb")
		assert.Equal(t, "systemctl enable postgresql", pkg.PostInstall[0].RunCommands[1])
		assert.Equal(t, "systemctl start postgresql", pkg.PostInstall[0].RunCommands[2])
		require.Len(t, pkg.PostInstall[1].RunCommands, 3)
		assert.Contains(t, pkg.PostInstall[1].RunCommands[0], `configValue "db-root-password"`)
	})

	t.Run("redis-server maps to valkey", func(t *testing.T) {
		pkg, exists := packages["redis-server"]
		require.True(t, exists, "redis-server should exist in default.yaml")
		assert.Equal(t, []string{"valkey"}, pkg.ReplaceWith)
		require.Len(t, pkg.PostInstall, 1)
		require.Len(t, pkg.PostInstall[0].RunCommands, 2)
		assert.Equal(t, "systemctl enable valkey", pkg.PostInstall[0].RunCommands[0])
		assert.Equal(t, "systemctl start valkey", pkg.PostInstall[0].RunCommands[1])
	})

	t.Run("docker native package with service", func(t *testing.T) {
		pkg, exists := packages["docker"]
		require.True(t, exists, "docker should exist in default.yaml")
		assert.Equal(t, []string{"docker"}, pkg.ReplaceWith)
		assert.Equal(t, []string{"docker"}, pkg.LookupPaths)
		require.Len(t, pkg.PostInstall, 1)
		assert.Equal(t, "systemctl enable docker", pkg.PostInstall[0].RunCommands[0])
		assert.Equal(t, "systemctl start docker", pkg.PostInstall[0].RunCommands[1])
	})

	t.Run("go native package", func(t *testing.T) {
		pkg, exists := packages["go"]
		require.True(t, exists, "go should exist in default.yaml")
		assert.Equal(t, []string{"go"}, pkg.ReplaceWith)
		assert.Equal(t, []string{"go"}, pkg.LookupPaths)
	})

	t.Run("nodejs maps to nodejs and npm", func(t *testing.T) {
		pkg, exists := packages["nodejs"]
		require.True(t, exists, "nodejs should exist in default.yaml")
		assert.Equal(t, []string{"nodejs", "npm"}, pkg.ReplaceWith)
		assert.Equal(t, []string{"node", "npm"}, pkg.LookupPaths)
	})
}

func TestLoadPackages_DNF_CentOS7(t *testing.T) {
	packages, err := LoadPackages("dnf", osinfo.Info{
		Distribution:        osinfo.DistributionCentOS,
		DistributionVersion: "7",
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("php with pre-install", func(t *testing.T) {
		pkg, exists := packages["php"]
		require.True(t, exists, "php should exist in centos_7.yaml")
		assert.Equal(t, "php", pkg.Name)
		assert.Equal(t, []string{"php-cli", "php-common", "php-fpm"}, pkg.ReplaceWith)
		require.Len(t, pkg.PreInstall, 1)
		require.Len(t, pkg.PreInstall[0].RunCommands, 3)
		assert.Equal(t, "yum -y install https://rpms.remirepo.net/enterprise/remi-release-7.rpm", pkg.PreInstall[0].RunCommands[0])
		assert.Equal(t, "yum -y install yum-utils", pkg.PreInstall[0].RunCommands[1])
		assert.Equal(t, "yum-config-manager --enable remi-php82", pkg.PreInstall[0].RunCommands[2])
	})

	t.Run("inherits from default.yaml", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should be inherited from default.yaml")
		assert.Equal(t, []string{"libgcc.i686"}, pkg.ReplaceWith)
	})
}

func TestLoadPackages_DNF_CentOS8(t *testing.T) {
	packages, err := LoadPackages("dnf", osinfo.Info{
		Distribution:        osinfo.DistributionCentOS,
		DistributionVersion: "8",
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("php with pre-install", func(t *testing.T) {
		pkg, exists := packages["php"]
		require.True(t, exists, "php should exist in centos_8.yaml")
		assert.Equal(t, "php", pkg.Name)
		assert.Equal(t, []string{"php-cli", "php-common", "php-fpm"}, pkg.ReplaceWith)
		require.Len(t, pkg.PreInstall, 1)
		require.Len(t, pkg.PreInstall[0].RunCommands, 2)
		assert.Equal(t, "dnf -y install https://rpms.remirepo.net/enterprise/remi-release-8.rpm", pkg.PreInstall[0].RunCommands[0])
		assert.Equal(t, "dnf -y module switch-to php:remi-8.2", pkg.PreInstall[0].RunCommands[1])
	})
}

func TestLoadPackages_DNF_CentOS10(t *testing.T) {
	packages, err := LoadPackages("dnf", osinfo.Info{
		Distribution:        osinfo.DistributionCentOS,
		DistributionVersion: "10",
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("redis-server with pre-install and post-install", func(t *testing.T) {
		pkg, exists := packages["redis-server"]
		require.True(t, exists, "redis-server should exist in centos_10.yaml")
		assert.Equal(t, "redis-server", pkg.Name)
		assert.Equal(t, []string{"redis"}, pkg.ReplaceWith)

		require.Len(t, pkg.PreInstall, 1)
		require.Len(t, pkg.PreInstall[0].RunCommands, 2)
		assert.Equal(t, "dnf install -y epel-release", pkg.PreInstall[0].RunCommands[0])
		assert.Equal(t, "dnf module enable redis:remi-7.2 -y", pkg.PreInstall[0].RunCommands[1])

		require.Len(t, pkg.PostInstall, 1)
		require.Len(t, pkg.PostInstall[0].RunCommands, 2)
		assert.Equal(t, "systemctl enable redis", pkg.PostInstall[0].RunCommands[0])
		assert.Equal(t, "systemctl start redis", pkg.PostInstall[0].RunCommands[1])
	})

	t.Run("postgresql overrides default", func(t *testing.T) {
		pkg, exists := packages["postgresql"]
		require.True(t, exists, "postgresql should exist")
		assert.Equal(t, []string{"postgresql-server", "postgresql-contrib"}, pkg.ReplaceWith)
		require.Len(t, pkg.PostInstall, 2)
		require.Len(t, pkg.PostInstall[0].RunCommands, 3)
		assert.Equal(t, "postgresql-setup --initdb", pkg.PostInstall[0].RunCommands[0])
	})
}

func TestLoadPackages_DNF_Rocky(t *testing.T) {
	packages, err := LoadPackages("dnf", osinfo.Info{
		Distribution: osinfo.DistributionRocky,
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("lib32gcc overridden with empty array", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in rocky.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Empty(t, pkg.ReplaceWith, "rocky.yaml overrides with empty array")
	})

	t.Run("lib32stdc6 overridden with empty array", func(t *testing.T) {
		pkg, exists := packages["lib32stdc6"]
		require.True(t, exists, "lib32stdc6 should exist in rocky.yaml")
		assert.Empty(t, pkg.ReplaceWith, "rocky.yaml overrides with empty array")
	})

	t.Run("lib32z1 overridden with empty array", func(t *testing.T) {
		pkg, exists := packages["lib32z1"]
		require.True(t, exists, "lib32z1 should exist in rocky.yaml")
		assert.Empty(t, pkg.ReplaceWith, "rocky.yaml overrides with empty array")
	})

	t.Run("inherits packages not in rocky.yaml", func(t *testing.T) {
		pkg, exists := packages["postgresql"]
		require.True(t, exists, "postgresql should be inherited from default.yaml")
		assert.Equal(t, []string{"postgresql-server", "postgresql-contrib"}, pkg.ReplaceWith)
	})
}

func TestLoadPackages_DNF_MergeOverride(t *testing.T) {
	t.Run("CentOS 10 redis-server overrides default", func(t *testing.T) {
		packages, err := LoadPackages("dnf", osinfo.Info{
			Distribution:        osinfo.DistributionCentOS,
			DistributionVersion: "10",
		})
		require.NoError(t, err)

		pkg, exists := packages["redis-server"]
		require.True(t, exists)

		require.Len(t, pkg.PreInstall, 1, "CentOS 10 should have pre-install steps")
		require.Len(t, pkg.PreInstall[0].RunCommands, 2, "CentOS 10 should have pre-install commands")
		assert.Contains(t, pkg.PreInstall[0].RunCommands[0], "epel-release")

		require.Len(t, pkg.PostInstall, 1, "CentOS 10 should override default post-install")
		require.Len(t, pkg.PostInstall[0].RunCommands, 2, "CentOS 10 should have post-install commands")
	})
}

func TestLoadPackages_DNF_NonExistentDistribution(t *testing.T) {
	packages, err := LoadPackages("dnf", osinfo.Info{
		Distribution:        "nonexistent",
		DistributionVersion: "999",
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages, "should still load default.yaml")

	_, exists := packages["lib32gcc"]
	assert.True(t, exists, "should have default packages")
}

func TestLoadPackages_DNF_CaseInsensitiveDistribution(t *testing.T) {
	packagesLower, err := LoadPackages("dnf", osinfo.Info{
		Distribution:        osinfo.DistributionCentOS,
		DistributionVersion: "7",
	})
	require.NoError(t, err)

	packagesUpper, err := LoadPackages("dnf", osinfo.Info{
		Distribution:        "CentOS",
		DistributionVersion: "7",
	})
	require.NoError(t, err)

	assert.Equal(t, len(packagesLower), len(packagesUpper), "should load same files regardless of case")
}

func TestReplaceDistributionVariables(t *testing.T) {
	osinf := osinfo.Info{
		Distribution:         osinfo.DistributionDebian,
		DistributionVersion:  "12",
		DistributionCodename: "bookworm",
		Platform:             osinfo.PlatformAmd64,
	}

	t.Run("replace all variables", func(t *testing.T) {
		input := "deb {{distname}} {{distversion}} {{codename}} {{architecture}}"
		expected := "deb debian 12 bookworm amd64"
		assert.Equal(t, expected, replaceDistributionVariables(input, osinf))
	})

	t.Run("replace no variables", func(t *testing.T) {
		input := "simple string"
		expected := "simple string"
		assert.Equal(t, expected, replaceDistributionVariables(input, osinf))
	})

	t.Run("replace single variable", func(t *testing.T) {
		input := "Distribution: {{distname}}"
		expected := "Distribution: debian"
		assert.Equal(t, expected, replaceDistributionVariables(input, osinf))
	})

	t.Run("replace multiple occurrences", func(t *testing.T) {
		input := "{{distname}} {{distname}} {{distname}}"
		expected := "debian debian debian" //nolint:dupword
		assert.Equal(t, expected, replaceDistributionVariables(input, osinf))
	})
}

func TestReplaceDistributionVariablesSlice(t *testing.T) {
	osinf := osinfo.Info{
		Distribution:         osinfo.DistributionDebian,
		DistributionVersion:  "12",
		DistributionCodename: "bookworm",
		Platform:             osinfo.PlatformAmd64,
	}

	t.Run("replace variables in slice", func(t *testing.T) {
		inputs := []string{
			"deb {{distname}}",
			"version {{distversion}}",
			"codename {{codename}}",
			"arch {{architecture}}",
		}
		expected := []string{
			"deb debian",
			"version 12",
			"codename bookworm",
			"arch amd64",
		}
		result := replaceDistributionVariablesSlice(inputs, osinf)
		assert.Equal(t, expected, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		inputs := []string{}
		result := replaceDistributionVariablesSlice(inputs, osinf)
		assert.Empty(t, result)
	})

	t.Run("slice with no variables", func(t *testing.T) {
		inputs := []string{"foo", "bar", "baz"}
		expected := []string{"foo", "bar", "baz"}
		result := replaceDistributionVariablesSlice(inputs, osinf)
		assert.Equal(t, expected, result)
	})
}

func Test_normalizeCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line command",
			input:    "apt-get install -y postgresql",
			expected: "apt-get install -y postgresql",
		},
		{
			name: "multiline command with newlines",
			input: `curl -fsSL https://example.com/key.gpg
  | gpg --dearmor
  -o /usr/share/keyrings/example.gpg`,
			expected: `curl -fsSL https://example.com/key.gpg | gpg --dearmor -o /usr/share/keyrings/example.gpg`,
		},
		{
			name:     "multiple consecutive spaces",
			input:    "apt-get    install     -y  package",
			expected: "apt-get install -y package",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  apt-get update  ",
			expected: "apt-get update",
		},
		{
			name:     "tabs and spaces mixed",
			input:    "echo\t\ttest\tcommand",
			expected: "echo test command",
		},
		{
			name:     "windows line endings (CRLF)",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1 line2 line3",
		},
		{
			name:     "unix line endings (LF)",
			input:    "line1\nline2\nline3",
			expected: "line1 line2 line3",
		},
		{
			name:     "old mac line endings (CR)",
			input:    "line1\rline2\rline3",
			expected: "line1 line2 line3",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\t\r\n   ",
			expected: "",
		},
		{
			name: "real command from yaml",
			input: `curl -sS https://getcomposer.org/installer
  | php --
    --install-dir=/usr/local/bin
    --filename=composer`,
			expected: `curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer`,
		},
		{
			name: "real dnf command from yaml",
			input: `dnf -y install https://rpms.remirepo.net/enterprise/remi-release-8.rpm
  && dnf -y module switch-to php:remi-8.2`,
			expected: `dnf -y install https://rpms.remirepo.net/enterprise/remi-release-8.rpm && dnf -y module switch-to php:remi-8.2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCommand(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
