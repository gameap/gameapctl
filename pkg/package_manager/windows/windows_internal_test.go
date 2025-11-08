package windows

import (
	"testing"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPackages(t *testing.T) {
	t.Run("load default packages", func(t *testing.T) {
		info := osinfo.Info{
			Distribution: "windows",
			Platform:     "amd64",
		}

		packages, err := LoadPackages(info)
		require.NoError(t, err)
		assert.NotEmpty(t, packages)

		assert.Contains(t, packages, "nginx")
		assert.Contains(t, packages, "php")
	})

	t.Run("packages have correct structure", func(t *testing.T) {
		info := osinfo.Info{
			Distribution: "windows",
			Platform:     "amd64",
		}

		packages, err := LoadPackages(info)
		require.NoError(t, err)

		nginx, exists := packages["nginx"]
		require.True(t, exists)
		assert.Equal(t, "nginx", nginx.Name)
		assert.NotEmpty(t, nginx.LookupPaths)
		assert.NotEmpty(t, nginx.DownloadURLs)

		if nginx.Service != nil {
			assert.NotEmpty(t, nginx.Service.Executable)
		}
	})

	t.Run("load with empty info", func(t *testing.T) {
		info := osinfo.Info{}

		packages, err := LoadPackages(info)
		require.NoError(t, err)
		assert.NotEmpty(t, packages)
	})
}

func Test_replaceValues(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		osInfo osinfo.Info
		pkg    Package
		want   string
	}{
		{
			name:  "replace architecture",
			input: "download-{{architecture}}.zip",
			osInfo: osinfo.Info{
				Platform: "amd64",
			},
			pkg:  Package{},
			want: "download-amd64.zip",
		},
		{
			name:  "replace distname",
			input: "{{distname}}-installer.exe",
			osInfo: osinfo.Info{
				Distribution: "windows",
			},
			pkg:  Package{},
			want: "windows-installer.exe",
		},
		{
			name:  "replace distversion",
			input: "version-{{distversion}}",
			osInfo: osinfo.Info{
				DistributionVersion: "10",
			},
			pkg:  Package{},
			want: "version-10",
		},
		{
			name:  "replace codename",
			input: "{{codename}}-release",
			osInfo: osinfo.Info{
				DistributionCodename: "21H2",
			},
			pkg:  Package{},
			want: "21H2-release",
		},
		{
			name:   "replace package install path",
			input:  "{{package_install_path}}\\config.ini",
			osInfo: osinfo.Info{},
			pkg: Package{
				InstallPath: "C:\\Program Files\\App",
			},
			want: "C:\\Program Files\\App\\config.ini",
		},
		{
			name:  "replace multiple placeholders",
			input: "{{distname}}-{{architecture}}-{{distversion}}",
			osInfo: osinfo.Info{
				Distribution:        "windows",
				Platform:            "amd64",
				DistributionVersion: "10",
			},
			pkg:  Package{},
			want: "windows-amd64-10",
		},
		{
			name:  "no placeholders",
			input: "plain-text",
			osInfo: osinfo.Info{
				Distribution: "windows",
			},
			pkg:  Package{},
			want: "plain-text",
		},
		{
			name:   "empty string",
			input:  "",
			osInfo: osinfo.Info{},
			pkg:    Package{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceValues(tt.input, tt.osInfo, tt.pkg)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_replaceValuesSlice(t *testing.T) {
	t.Run("replace in slice", func(t *testing.T) {
		input := []string{
			"download-{{architecture}}.zip",
			"{{distname}}-installer.exe",
			"plain-text",
		}
		osInfo := osinfo.Info{
			Distribution: "windows",
			Platform:     "amd64",
		}
		pkg := Package{}

		result := replaceValuesSlice(input, osInfo, pkg)
		require.Len(t, result, 3)
		assert.Equal(t, "download-amd64.zip", result[0])
		assert.Equal(t, "windows-installer.exe", result[1])
		assert.Equal(t, "plain-text", result[2])
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []string{}
		osInfo := osinfo.Info{}
		pkg := Package{}

		result := replaceValuesSlice(input, osInfo, pkg)
		assert.Empty(t, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var input []string
		osInfo := osinfo.Info{}
		pkg := Package{}

		result := replaceValuesSlice(input, osInfo, pkg)
		assert.Empty(t, result)
	})
}

func TestPackageStructure(t *testing.T) {
	t.Run("package with service", func(t *testing.T) {
		pkg := Package{
			Name:        "test-service",
			LookupPaths: []string{"test.exe"},
			Service: &Service{
				ID:         "test",
				Name:       "Test Service",
				Executable: "test.exe",
				Arguments:  "--port 8080",
				Env: []EnvironmentVar{
					{Name: "TEST_VAR", Value: "test_value"},
				},
			},
		}

		assert.Equal(t, "test-service", pkg.Name)
		require.NotNil(t, pkg.Service)
		assert.Equal(t, "test", pkg.Service.ID)
		require.Len(t, pkg.Service.Env, 1)
		assert.Equal(t, "TEST_VAR", pkg.Service.Env[0].Name)
	})

	t.Run("package with service account", func(t *testing.T) {
		pkg := Package{
			Name: "test",
			Service: &Service{
				ID:         "test",
				Name:       "Test",
				Executable: "test.exe",
				ServiceAccount: &ServiceAccount{
					Username: "DOMAIN\\User",
					Password: "password123",
				},
			},
		}

		require.NotNil(t, pkg.Service)
		require.NotNil(t, pkg.Service.ServiceAccount)
		assert.Equal(t, "DOMAIN\\User", pkg.Service.ServiceAccount.Username)
		assert.Equal(t, "password123", pkg.Service.ServiceAccount.Password)
	})

	t.Run("package with dependencies", func(t *testing.T) {
		pkg := Package{
			Name:         "app",
			Dependencies: []string{"dep1", "dep2", "dep3"},
		}

		require.Len(t, pkg.Dependencies, 3)
		assert.Equal(t, "dep1", pkg.Dependencies[0])
		assert.Equal(t, "dep2", pkg.Dependencies[1])
		assert.Equal(t, "dep3", pkg.Dependencies[2])
	})

	t.Run("package with pre-install steps", func(t *testing.T) {
		pkg := Package{
			Name: "app",
			PreInstall: []PreInstall{
				{
					GrantPermissions: []Permission{
						{
							Path:   "C:\\app",
							User:   "NT AUTHORITY\\NETWORK SERVICE",
							Access: "full-control",
						},
					},
					Commands: []string{
						"mkdir C:\\app\\logs",
						"echo test > C:\\app\\config.txt",
					},
				},
			},
		}

		require.Len(t, pkg.PreInstall, 1)
		require.Len(t, pkg.PreInstall[0].GrantPermissions, 1)
		assert.Equal(t, "C:\\app", pkg.PreInstall[0].GrantPermissions[0].Path)
		require.Len(t, pkg.PreInstall[0].Commands, 2)
	})

	t.Run("package with install steps", func(t *testing.T) {
		pkg := Package{
			Name: "app",
			Install: []InstallStep{
				{
					RunCommands:    []string{"install.bat"},
					WaitForService: "AppService",
				},
				{
					RunCommands:             []string{"configure.bat"},
					WaitForFiles:            []string{"C:\\app\\config.json"},
					AllowedInstallExitCodes: []int{0, 3010},
				},
			},
		}

		require.Len(t, pkg.Install, 2)
		assert.Equal(t, "AppService", pkg.Install[0].WaitForService)
		require.Len(t, pkg.Install[1].WaitForFiles, 1)
		assert.Equal(t, "C:\\app\\config.json", pkg.Install[1].WaitForFiles[0])
		require.Len(t, pkg.Install[1].AllowedInstallExitCodes, 2)
	})
}
