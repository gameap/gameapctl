package packagemanager

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/gopherclass/go-shellquote"
	"github.com/pkg/errors"
)

type pack struct {
	DownloadPathes []string
	LookupPath     []string
	InstallCommand string
}

var repository = map[string]pack{
	NginxPackage: {
		LookupPath: []string{"nginx"},
		DownloadPathes: []string{
			"http://nginx.org/download/nginx-1.22.1.zip",
		},
	},
	MariaDBServerPackage: {
		LookupPath: []string{"mysql", "mariadb"},
		DownloadPathes: []string{
			"https://mirror.23m.com/mariadb/mariadb-10.6.11/winx64-packages/mariadb-10.6.11-winx64.msi",
			"https://ftp.bme.hu/pub/mirrors/mariadb/mariadb-10.6.11/winx64-packages/mariadb-10.6.11-winx64.msi",
		},
		InstallCommand: "msiexec /i mariadb-10.6.11-winx64.msi SERVICENAME=MariaDB PORT=3306 /qn",
	},
	PHPPackage: {
		LookupPath: []string{"php"},
		DownloadPathes: []string{
			"https://windows.php.net/downloads/releases/php-8.2.1-Win32-vs16-x64.zip",
		},
	},
}

type WindowsPackageManager struct {
}

func NewWindowsPackageManager() *WindowsPackageManager {
	return &WindowsPackageManager{}
}

func (pm *WindowsPackageManager) Search(_ context.Context, _ string) ([]PackageInfo, error) {
	return nil, nil
}

func (pm *WindowsPackageManager) Install(ctx context.Context, packs ...string) error {
	var err error
	for _, p := range packs {
		repoPack, exists := repository[p]
		if !exists {
			continue
		}

		err = pm.installPackage(ctx, p, repoPack)
		if err != nil {
			return err
		}
	}

	return nil
}

func (pm *WindowsPackageManager) installPackage(ctx context.Context, packName string, p pack) error {
	var err error

	packagePath := ""
	for _, c := range p.LookupPath {
		packagePath, err = exec.LookPath(c)
		if err != nil {
			continue
		}

		log.Printf("Package %s is found in path '%s'\n", p, filepath.Dir(packagePath))
		break
	}

	processor, ok := packageProcessors[packName]
	if ok {
		err = processor(ctx, packagePath)
		if err != nil {
			return err
		}
	}

	if packagePath != "" {
		return nil
	}

	dir, err := os.MkdirTemp("", "install")
	if err != nil {
		return errors.WithMessagef(err, "failed to make temp directory")
	}

	for _, path := range p.DownloadPathes {
		err = utils.DownloadFile(ctx, path, dir)
		if err != nil {
			log.Println("failed to download file")
			log.Println(err)
			continue
		}

		break
	}

	if p.InstallCommand == "" {
		return errors.New("empty install command for package")
	}

	cmd, err := shellquote.Split(p.InstallCommand)
	if err != nil {
		return errors.WithMessage(err, "failed to split command")
	}

	return utils.ExecCommand(cmd[0], cmd[1:]...)
}

func (pm *WindowsPackageManager) CheckForUpdates(_ context.Context) error {
	return nil
}

func (pm *WindowsPackageManager) Remove(_ context.Context, _ ...string) error {
	return errors.New("removing packages is not supported on Windows")
}

func (pm *WindowsPackageManager) Purge(_ context.Context, _ ...string) error {
	return errors.New("removing packages is not supported on Windows")
}

var packageProcessors = map[string]func(ctx context.Context, packagePath string) error{
	//nolint
	PHPExtensionsPackage: func(_ context.Context, _ string) error {
		_ = utils.ExecCommand("php", "-r", "echo php_ini_loaded_file();")
		_ = utils.ExecCommand("php", "-r", "echo php_ini_scanned_files();")
		return nil
	},
}
