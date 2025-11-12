package dnf

import (
	"testing"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPackages_Default(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{})
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	t.Run("lib32gcc package", func(t *testing.T) {
		pkg, exists := packages["lib32gcc"]
		require.True(t, exists, "lib32gcc should exist in default.yaml")
		assert.Equal(t, "lib32gcc", pkg.Name)
		assert.Equal(t, []string{"libgcc.i686"}, pkg.ReplaceWith)
		assert.False(t, pkg.Virtual)
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

	t.Run("php-extensions virtual package", func(t *testing.T) {
		pkg, exists := packages["php-extensions"]
		require.True(t, exists, "php-extensions should exist in default.yaml")
		assert.Equal(t, "php-extensions", pkg.Name)
		assert.True(t, pkg.Virtual)
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

func TestLoadPackages_CentOS7(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_CentOS8(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_CentOS10(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_Rocky(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_MergeOverride(t *testing.T) {
	t.Run("CentOS 10 redis-server overrides default", func(t *testing.T) {
		packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_NonExistentDistribution(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
		Distribution:        "nonexistent",
		DistributionVersion: "999",
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages, "should still load default.yaml")

	_, exists := packages["lib32gcc"]
	assert.True(t, exists, "should have default packages")
}

func TestLoadPackages_CaseInsensitiveDistribution(t *testing.T) {
	packagesLower, err := LoadPackages(osinfo.Info{
		Distribution:        osinfo.DistributionCentOS,
		DistributionVersion: "7",
	})
	require.NoError(t, err)

	packagesUpper, err := LoadPackages(osinfo.Info{
		Distribution:        "CentOS",
		DistributionVersion: "7",
	})
	require.NoError(t, err)

	assert.Equal(t, len(packagesLower), len(packagesUpper), "should load same files regardless of case")
}

func Test_normalizeCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line command",
			input:    "dnf install -y postgresql",
			expected: "dnf install -y postgresql",
		},
		{
			name: "multiline command with newlines",
			input: `systemctl enable postgresql &&
  systemctl start postgresql &&
  systemctl status postgresql`,
			expected: `systemctl enable postgresql && systemctl start postgresql && systemctl status postgresql`,
		},
		{
			name:     "multiple consecutive spaces",
			input:    "dnf    install     -y  package",
			expected: "dnf install -y package",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  dnf update  ",
			expected: "dnf update",
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
