package packagemanager

import (
	"bytes"
	"context"
	"encoding/xml"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/gopherclass/go-shellquote"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type pack struct {
	DownloadURLs            []string
	LookupPath              []string
	InstallCommand          string
	AllowedInstallExitCodes []int
	DefaultInstallPath      string
	ServiceConfig           *WinSWServiceConfig
	Dependencies            []string
}

const (
	WinSWPackage      = "winsw"
	VCRedist16Package = "vc_redist_16" //nolint:gosec
)

const servicesConfigPath = "C:\\gameap\\services"

var repository = map[string]pack{
	NginxPackage: {
		LookupPath: []string{"nginx"},
		DownloadURLs: []string{
			"http://nginx.org/download/nginx-1.22.1.zip",
		},
		DefaultInstallPath: "C:\\gameap\\tools\\nginx",
		ServiceConfig: &WinSWServiceConfig{
			ID:               "nginx",
			Name:             "nginx",
			Executable:       "nginx",
			WorkingDirectory: "C:\\gameap\\tools\\nginx",
			StopExecutable:   "nginx",
			StopArguments:    "-s stop",
			OnFailure: []onFailure{
				{Action: "restart", Delay: "1 sec"},
				{Action: "restart", Delay: "2 sec"},
				{Action: "restart", Delay: "5 sec"},
				{Action: "restart", Delay: "5 sec"},
			},
			ResetFailure: "1 hour",
		},
	},
	MySQLServerPackage: {
		LookupPath: []string{"mariadb"},
		DownloadURLs: []string{
			"https://mirror.23m.com/mariadb/mariadb-10.6.12/winx64-packages/mariadb-10.6.12-winx64.msi",
			"https://ftp.bme.hu/pub/mirrors/mariadb/mariadb-10.6.12/winx64-packages/mariadb-10.6.12-winx64.msi",
		},
		InstallCommand: "cmd /c \"start /wait msiexec /i mariadb-10.6.12-winx64.msi SERVICENAME=MariaDB PORT=9306 /qb\"",
	},
	PHPPackage: {
		LookupPath: []string{"php"},
		DownloadURLs: []string{
			"https://windows.php.net/downloads/releases/php-8.2.2-Win32-vs16-x64.zip",
			"https://windows.php.net/downloads/releases/php-7.4.33-nts-Win32-vc15-x64.zip",
		},
		DefaultInstallPath: "C:\\php",
		ServiceConfig: &WinSWServiceConfig{
			ID:         "php-fpm",
			Name:       "php-fpm",
			Executable: "php-cgi",
			Arguments:  "-b 127.0.0.1:9934 -c C:\\php\\php.ini",
			OnFailure: []onFailure{
				{Action: "restart", Delay: "1 sec"},
				{Action: "restart", Delay: "2 sec"},
				{Action: "restart", Delay: "5 sec"},
				{Action: "restart", Delay: "5 sec"},
			},
			ResetFailure: "1 hour",
		},
		Dependencies: []string{VCRedist16Package},
	},
	PHPExtensionsPackage: {
		LookupPath: []string{"php"},
	},
	VCRedist16Package: {
		DownloadURLs: []string{
			"https://aka.ms/vs/16/release/VC_redist.x64.exe",
		},
		InstallCommand: "cmd /c \"VC_redist.x64.exe /install /quiet /norestart\"",
		AllowedInstallExitCodes: []int{
			1638, // A newer version is already installed or already installed
		},
	},
	WinSWPackage: {
		LookupPath: []string{"winsw"},
		DownloadURLs: []string{
			"https://github.com/winsw/winsw/releases/download/v3.0.0-alpha.11/WinSW-x64.exe",
		},
		DefaultInstallPath: "C:\\Windows\\System32",
		InstallCommand:     "cmd /c \"move WinSW-x64.exe winsw.exe\"",
	},
}

type WindowsPackageManager struct{}

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

//nolint:funlen,gocognit
func (pm *WindowsPackageManager) installPackage(ctx context.Context, packName string, p pack) error {
	log.Println("Installing", packName, "package")
	var err error

	packagePath := ""
	for _, c := range p.LookupPath {
		packagePath, err = exec.LookPath(c)
		if err != nil {
			continue
		}

		log.Printf("Package %s is found in path '%s'\n", packName, filepath.Dir(packagePath))

		break
	}

	if len(p.Dependencies) > 0 {
		for _, d := range p.Dependencies {
			err = pm.Install(ctx, d)
			if err != nil {
				return errors.WithMessagef(err, "failed to install dependency '%s'", d)
			}
		}
	}

	preProcessor, ok := packagePreProcessors[packName]
	if ok {
		log.Println("Execute pre processor for ", packName)
		err = preProcessor(ctx, packagePath)
		if err != nil {
			return err
		}
	}

	if packagePath != "" {
		if p.ServiceConfig != nil {
			err = pm.installService(ctx, packName, p)
			if err != nil {
				return errors.WithMessage(err, "failed to install service")
			}
		}

		log.Printf("Package path is not empty (%s), skipping for '%s' package \n", packagePath, packName)

		return nil
	}

	dir := p.DefaultInstallPath

	if dir == "" {
		dir, err = os.MkdirTemp("", "install")
		if err != nil {
			return errors.WithMessagef(err, "failed to make temp directory")
		}
		defer func(path string) {
			err := os.RemoveAll(path)
			if err != nil {
				log.Println(err)
			}
		}(dir)
	}

	for _, path := range p.DownloadURLs {
		log.Println("Downloading file from", path, "to", dir)

		err = utils.Download(ctx, path, dir)
		if err != nil {
			log.Println("failed to download file")
			log.Println(err)

			continue
		}

		err = nil

		break
	}
	if err != nil {
		return errors.WithMessage(err, "failed to download file")
	}

	if p.InstallCommand != "" {
		log.Println("Running install command for package ", packName)
		splitted, err := shellquote.Split(p.InstallCommand)
		if err != nil {
			return errors.WithMessage(err, "failed to split command")
		}

		//nolint:gosec
		cmd := exec.Command(splitted[0], splitted[1:]...)
		cmd.Stdout = log.Writer()
		cmd.Stderr = log.Writer()
		cmd.Dir = dir
		log.Println('\n', cmd.String())
		err = cmd.Run()
		if err != nil {
			if len(p.AllowedInstallExitCodes) > 0 && lo.Contains(p.AllowedInstallExitCodes, cmd.ProcessState.ExitCode()) {
				log.Println(errors.WithMessage(err, "failed to execute install command"))
				log.Println("Exit code is allowed")

				return nil
			}

			return errors.WithMessage(err, "failed to execute install command")
		}
	}

	postProcessor, ok := packagePostProcessors[packName]
	if ok {
		err = postProcessor(ctx, packagePath)
		if err != nil {
			return err
		}
	}

	if p.ServiceConfig != nil {
		err = pm.installService(ctx, packName, p)
		if err != nil {
			return errors.WithMessage(err, "failed to install service")
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

//nolint:funlen
func (pm *WindowsPackageManager) installService(ctx context.Context, packName string, p pack) error {
	_, err := exec.LookPath(repository[WinSWPackage].LookupPath[0])
	if err != nil {
		err = pm.Install(ctx, WinSWPackage)
		if err != nil {
			return errors.WithMessage(err, "failed to install winsw")
		}
	}

	log.Println("Installing service for package", packName)

	if service.IsExists(ctx, packName) {
		log.Printf("Service '%s' is already exists", packName)

		return nil
	}

	serviceConfig := *p.ServiceConfig

	if serviceConfig.WorkingDirectory == "" {
		path, err := exec.LookPath(serviceConfig.Executable)
		if err != nil {
			return errors.WithMessage(err, "failed to look path for service executable")
		}

		if path == "" {
			return errors.New("executable path not found")
		}

		serviceConfig.WorkingDirectory = filepath.Dir(path)
	}

	out, err := xml.Marshal(struct {
		WinSWServiceConfig
		XMLName struct{} `xml:"service"`
	}{WinSWServiceConfig: serviceConfig})

	if err != nil {
		return errors.WithMessage(err, "failed to marshal service config")
	}

	if _, err = os.Stat(servicesConfigPath); errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(servicesConfigPath, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create services config directory")
		}
	}

	configPath := filepath.Join(servicesConfigPath, packName+".xml")

	log.Println("create service config")

	err = utils.WriteContentsToFile(out, configPath)
	if err != nil {
		return errors.WithMessagef(err, "failed to save config for service '%s' ", packName)
	}

	err = utils.ExecCommand("winsw", "install", configPath)
	if err != nil {
		return errors.WithMessagef(err, "failed to install service '%s'", packName)
	}

	return nil
}

var packagePreProcessors = map[string]func(ctx context.Context, packagePath string) error{
	PHPExtensionsPackage: func(ctx context.Context, packagePath string) error {
		p := repository[PHPExtensionsPackage]

		cmd := exec.Command("php", "-r", "echo php_ini_scanned_files();")
		buf := &bytes.Buffer{}
		buf.Grow(100) //nolint:gomnd
		cmd.Stdout = buf
		cmd.Stderr = log.Writer()
		log.Println("\n", cmd.String())
		err := cmd.Run()
		if err != nil {
			return errors.WithMessage(err, "failed to get scanned files")
		}

		scannedFiles := strings.Split(buf.String(), "\n")

		if len(scannedFiles) > 0 {
			firstScannedFile := strings.TrimSpace(scannedFiles[0])
			scannedFileDir := filepath.Dir(firstScannedFile)

			exts := []string{
				"bz2", "curl", "fileinfo", "gd2", "gmp", "intl",
				"mbstring", "openssl", "pdo_mysql", "pdo_sqlite", "zip",
			}

			for _, e := range exts {
				err = utils.WriteContentsToFile([]byte(`extension=`+e), filepath.Join(scannedFileDir, e+".ini"))
				if err != nil {
					return errors.WithMessagef(err, "failed to create ini for '%s' php extension", e)
				}
			}
		}

		cmd = exec.Command("php", "-r", "echo php_ini_loaded_file();")
		buf = &bytes.Buffer{}
		buf.Grow(100) //nolint:gomnd
		cmd.Stdout = buf
		cmd.Stderr = log.Writer()
		log.Println("\n", cmd.String())
		err = cmd.Run()
		if err != nil {
			return errors.WithMessage(err, "failed to get ini loaded file from php")
		}
		loadedFiles := strings.Split(buf.String(), "\n")
		iniFilePath := ""
		if len(loadedFiles) > 0 {
			iniFilePath = strings.TrimSpace(loadedFiles[0])
		}
		if iniFilePath == "" {
			if packagePath == "" {
				iniFilePath = filepath.Join(p.DefaultInstallPath, "php.ini")
			} else {
				iniFilePath = filepath.Join(filepath.Dir(packagePath), "php.ini")
			}
		}

		if !utils.IsFileExists(iniFilePath) {
			log.Println("Creating php.ini file on", iniFilePath)
			f, err := os.Create(iniFilePath)
			if err != nil {
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
		}

		if iniFilePath != "" {
			return utils.FindLineAndReplaceOrAdd(ctx, iniFilePath, map[string]string{
				";?\\s*extension=bz2\\s*":        "extension=bz2",
				";?\\s*extension=curl\\s*":       "extension=curl",
				";?\\s*extension=fileinfo\\s*":   "extension=fileinfo",
				";?\\s*extension=gd\\s*":         "extension=gd2",
				";?\\s*extension=gmp\\s*":        "extension=gmp",
				";?\\s*extension=intl\\s*":       "extension=intl",
				";?\\s*extension=mbstring\\s*":   "extension=mbstring",
				";?\\s*extension=openssl\\s*":    "extension=openssl",
				";?\\s*extension=pdo_mysql\\s*":  "extension=pdo_mysql",
				";?\\s*extension=pdo_sqlite\\s*": "extension=pdo_sqlite",
				";?\\s*extension=zip\\s*":        "extension=zip",
			})
		}

		return errors.New("failed to find config edition way to enable php extensions")
	},
}

var packagePostProcessors = map[string]func(ctx context.Context, packagePath string) error{
	PHPPackage: func(_ context.Context, _ string) error {
		p := repository[PHPPackage]

		path, _ := os.LookupEnv("PATH")

		return os.Setenv("PATH", path+string(os.PathListSeparator)+p.DefaultInstallPath)
	},
	NginxPackage: func(_ context.Context, _ string) error {
		p := repository[NginxPackage]

		entries, err := os.ReadDir(p.DefaultInstallPath)
		if err != nil {
			return err
		}

		if len(entries) != 1 {
			return NewErrInvalidDirContents(p.DefaultInstallPath)
		}

		d := filepath.Join(p.DefaultInstallPath, entries[0].Name())

		entries, err = os.ReadDir(d)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = utils.Move(filepath.Join(d, entry.Name()), filepath.Join(p.DefaultInstallPath, entry.Name()))
			if err != nil {
				return errors.WithMessage(err, "failed to move file")
			}
		}

		log.Println("Removing", d)

		return os.RemoveAll(d)
	},
}

type WinSWServiceConfig struct {
	ID               string `xml:"id"`
	Name             string `xml:"name"`
	Executable       string `xml:"executable"`
	WorkingDirectory string `xml:"workingdirectory,omitempty"`
	Arguments        string `xml:"arguments,omitempty"`

	StopExecutable string `xml:"stopexecutable,omitempty"`
	StopArguments  string `xml:"stoparguments,omitempty"`

	OnFailure    []onFailure `xml:"onfailure,omitempty"`
	ResetFailure string      `xml:"resetfailure,omitempty"`

	ServiceAccount struct {
		Username string `xml:"username,omitempty"`
		Password string `xml:"password,omitempty"`
	} `xml:"serviceaccount,omitempty"`
}

type onFailure struct {
	Action string `xml:"action,attr"`
	Delay  string `xml:"delay,attr,omitempty"`
}
