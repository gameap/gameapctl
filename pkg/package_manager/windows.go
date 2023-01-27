package packagemanager

import (
	"context"
	"log"
	"os"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/gopherclass/go-shellquote"
	"github.com/pkg/errors"
)

type WindowsPackageManager struct {
}

func NewWindowsPackageManager() *WindowsPackageManager {
	return &WindowsPackageManager{}
}

func (pm *WindowsPackageManager) Search(_ context.Context, _ string) ([]PackageInfo, error) {
	return nil, nil
}

func (pm *WindowsPackageManager) Install(ctx context.Context, packs ...string) error {
	for _, pack := range packs {
		repo, exists := repository[pack]
		if !exists {
			continue
		}

		dir, err := os.MkdirTemp("", "install")
		if err != nil {
			return errors.WithMessagef(err, "failed to make temp directory")
		}

		for _, path := range repo.DownloadPathes {
			err = utils.DownloadFile(ctx, path, dir)
			if err != nil {
				log.Println("failed to download file")
				log.Println(err)
				continue
			}

			break
		}

		cmd, err := shellquote.Split(repo.InstallCommand)
		if err != nil {
			return errors.WithMessage(err, "failed to split command")
		}
		err = utils.ExecCommand(cmd[0], cmd[1:]...)
		if err != nil {
			return err
		}
	}

	return nil
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

var repository = map[string]struct {
	DownloadPathes []string
	InstallCommand string
}{
	NginxPackage: {
		DownloadPathes: []string{
			"http://nginx.org/download/nginx-1.22.1.zip",
		},
	},
	MariaDBServerPackage: {
		DownloadPathes: []string{
			"https://mirror.23m.com/mariadb/mariadb-10.6.11/winx64-packages/mariadb-10.6.11-winx64.msi",
			"https://ftp.bme.hu/pub/mirrors/mariadb/mariadb-10.6.11/winx64-packages/mariadb-10.6.11-winx64.msi",
		},
		InstallCommand: "msiexec /i mariadb-10.6.11-winx64.msi SERVICENAME=MariaDB PORT=3306 /qn",
	},
	PHPPackage: {
		DownloadPathes: []string{
			"https://windows.php.net/downloads/releases/php-8.2.1-Win32-vs16-x64.zip",
		},
	},
}
