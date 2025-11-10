package apt

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

	t.Run("postgresql package", func(t *testing.T) {
		pkg, exists := packages["postgresql"]
		require.True(t, exists, "postgresql should exist in default.yaml")
		assert.Equal(t, "postgresql", pkg.Name)
		assert.Equal(t, []string{"postgresql", "postgresql-contrib"}, pkg.ReplaceWith)
		assert.False(t, pkg.Virtual)
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

	t.Run("composer virtual package with install", func(t *testing.T) {
		pkg, exists := packages["composer"]
		require.True(t, exists, "composer should exist in default.yaml")
		assert.Equal(t, "composer", pkg.Name)
		assert.True(t, pkg.Virtual)
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
		assert.Contains(t, pkg.PreInstall[1].RunCommands[0], "https://deb.nodesource.com/node_20.x")
	})
}

func TestLoadPackages_ARM64(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_Debian12(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_DebianSid(t *testing.T) {
	packages, err := LoadPackages(osinfo.Info{
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

func TestLoadPackages_FileHierarchy(t *testing.T) {
	t.Run("default overridden by arch-specific", func(t *testing.T) {
		packages, err := LoadPackages(osinfo.Info{
			Platform: osinfo.PlatformArm64,
		})
		require.NoError(t, err)

		pkg, exists := packages["lib32gcc"]
		require.True(t, exists)
		assert.Equal(t, []string{"lib32gcc-s1-amd64-cross"}, pkg.ReplaceWith)
	})

	t.Run("default overridden by distribution version", func(t *testing.T) {
		packages, err := LoadPackages(osinfo.Info{
			Distribution:        osinfo.DistributionDebian,
			DistributionVersion: "12",
		})
		require.NoError(t, err)

		pkg, exists := packages["lib32gcc"]
		require.True(t, exists)
		assert.Equal(t, []string{"lib32gcc-s1"}, pkg.ReplaceWith)
	})

	t.Run("default overridden by distribution codename", func(t *testing.T) {
		packages, err := LoadPackages(osinfo.Info{
			Distribution:         osinfo.DistributionDebian,
			DistributionCodename: "sid",
		})
		require.NoError(t, err)

		pkg, exists := packages["lib32gcc"]
		require.True(t, exists)
		assert.Equal(t, []string{"lib32gcc-s1"}, pkg.ReplaceWith)
	})
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCommand(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
