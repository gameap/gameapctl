package packagemanager

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	pathPkg "path"
	"path/filepath"
	"runtime"
	"strconv"
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

	PreInstallFunc func(ctx context.Context, p pack, resolvedPackagePath string) (pack, error)
}

const (
	WinSWPackage      = "winsw"
	VCRedist16Package = "vc_redist_16" //nolint:gosec
	GameAPDaemon      = "gameap-daemon"
)

const servicesConfigPath = "C:\\gameap\\services"

// https://curl.se/docs/caextract.html
const caCertURL = "https://curl.se/ca/cacert.pem"

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
		LookupPath: []string{"mysql", "mysqld"},
		DownloadURLs: []string{
			"https://archive.mariadb.org/mariadb-10.6.16/winx64-packages/mariadb-10.6.16-winx64.msi",
		},
		InstallCommand: "cmd /c \"start /wait msiexec /i mariadb-10.6.16-winx64.msi SERVICENAME=MariaDB PORT=9306 /qb\"",
	},
	MariaDBServerPackage: {
		LookupPath: []string{"mariadb", "mariadbd"},
		DownloadURLs: []string{
			"https://archive.mariadb.org/mariadb-10.6.16/winx64-packages/mariadb-10.6.16-winx64.msi",
		},
		InstallCommand: "cmd /c \"start /wait msiexec /i mariadb-10.6.16-winx64.msi SERVICENAME=MariaDB PORT=9306 /qb\"",
	},
	PHPPackage: {
		LookupPath: []string{"php"},
		DownloadURLs: []string{
			"https://windows.php.net/downloads/releases/php-8.3.3-Win32-vs16-x64.zip",
			"https://windows.php.net/downloads/releases/php-8.2.2-Win32-vs16-x64.zip",
		},
		DefaultInstallPath: "C:\\php",
		ServiceConfig: &WinSWServiceConfig{
			ID:         "php-fpm",
			Name:       "php-fpm",
			Executable: "php-cgi",
			Arguments:  "-b 127.0.0.1:9934 -c C:\\php\\php.ini",
			OnFailure: []onFailure{
				{Action: "restart"},
			},
			Env: []env{
				{Name: "PHP_FCGI_MAX_REQUESTS", Value: "0"},
				{Name: "PHP_FCGI_CHILDREN", Value: strconv.Itoa(runtime.NumCPU() * 2)},
			},
		},
		Dependencies: []string{VCRedist16Package},
		PreInstallFunc: func(ctx context.Context, p pack, path string) (pack, error) {
			if path != "" {
				p.ServiceConfig.Arguments = fmt.Sprintf(
					"-b 127.0.0.1:9934 -c %s",
					filepath.Join(filepath.Dir(path), "php.ini"),
				)
			}

			return p, nil
		},
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
	GitPackage: {
		LookupPath: []string{"git"},
		DownloadURLs: []string{
			"https://github.com/git-for-windows/git/releases/download/v2.43.0.windows.1/Git-2.43.0-64-bit.exe",
		},
		InstallCommand: "cmd /c Git-2.43.0-64-bit.exe " +
			"/VERYSILENT /NORESTART /NOCANCEL /SP- /CLOSEAPPLICATIONS " +
			"/RESTARTAPPLICATIONS /COMPONENTS=icons,ext\\reg\\shellhere,assoc,assoc_sh",
	},
	NodeJSPackage: {
		LookupPath: []string{"node"},
		DownloadURLs: []string{
			"https://nodejs.org/dist/v20.11.1/node-v20.11.1-x64.msi",
		},
		InstallCommand: "cmd /c start /wait msiexec /i node-v20.11.1-x64.msi /qb",
	},
	ComposerPackage: {
		LookupPath:   []string{"composer"},
		Dependencies: []string{PHPPackage},
		DownloadURLs: []string{
			"https://getcomposer.org/Composer-Setup.exe",
		},
		InstallCommand: "cmd /c Composer-Setup.exe /VERYSILENT /SUPPRESSMSGBOXES /ALLUSERS",
	},
	GameAPDaemon: {
		ServiceConfig: &WinSWServiceConfig{
			ID:               "GameAP Daemon",
			Name:             "GameAP Daemon",
			Executable:       "gameap-daemon",
			WorkingDirectory: "C:\\gameap\\daemon",
			OnFailure: []onFailure{
				{Action: "restart", Delay: "1 sec"},
				{Action: "restart", Delay: "2 sec"},
				{Action: "restart", Delay: "5 sec"},
				{Action: "restart", Delay: "5 sec"},
			},
			ResetFailure: "1 hour",
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

	resolvedPackagePath := ""
	for _, c := range p.LookupPath {
		resolvedPackagePath, err = exec.LookPath(c)
		if err != nil {
			continue
		}

		log.Printf("Package %s is found in path '%s'\n", packName, filepath.Dir(resolvedPackagePath))

		break
	}

	if p.PreInstallFunc != nil {
		p, err = p.PreInstallFunc(ctx, p, resolvedPackagePath)
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
		err = preProcessor(ctx, resolvedPackagePath)
		if err != nil {
			return err
		}
	}

	if resolvedPackagePath != "" {
		if p.ServiceConfig != nil {
			err = pm.installService(ctx, packName, p)
			if err != nil {
				return errors.WithMessage(err, "failed to install service")
			}
		}

		log.Printf("Package path is not empty (%s), skipping for '%s' package \n", resolvedPackagePath, packName)

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

		var parsedURL *url.URL
		parsedURL, err = url.Parse(path)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to parse url"))

			continue
		}

		if filepath.Ext(parsedURL.Path) == ".msi" {
			err = utils.DownloadFileOrArchive(
				ctx,
				path,
				filepath.Join(dir, pathPkg.Base(parsedURL.Path)),
			)
		} else {
			err = utils.Download(ctx, path, dir)
		}

		if err != nil {
			log.Println(errors.WithMessage(err, "failed to download file"))

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

	resolvedPackagePath = p.DefaultInstallPath

	postProcessor, ok := packagePostProcessors[packName]
	if ok {
		err = postProcessor(ctx, resolvedPackagePath)
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

	if !utils.IsFileExists(servicesConfigPath) {
		err = os.MkdirAll(servicesConfigPath, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create services config directory")
		}
	}

	configPath := filepath.Join(servicesConfigPath, packName+".xml")

	if utils.IsFileExists(configPath) {
		log.Printf("Service config for '%s' is already exists", packName)

		return nil
	}

	out, err := xml.MarshalIndent(struct {
		WinSWServiceConfig
		XMLName struct{} `xml:"service"`
	}{WinSWServiceConfig: serviceConfig}, "", "  ")

	if err != nil {
		return errors.WithMessage(err, "failed to marshal service config")
	}

	log.Println("Marshalled service config")
	log.Println(string(out))

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
				"bz2", "curl", "fileinfo", "gd", "gmp", "intl",
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

		if iniFilePath == "" {
			return errors.New("failed to find config edition way to enable php extensions")
		}

		err = utils.FindLineAndReplaceOrAdd(ctx, iniFilePath, map[string]string{
			";?\\s*extension=bz2\\s*":        "extension=bz2",
			";?\\s*extension=curl\\s*":       "extension=curl",
			";?\\s*extension=fileinfo\\s*":   "extension=fileinfo",
			";?\\s*extension=gd\\s*":         "extension=gd",
			";?\\s*extension=gmp\\s*":        "extension=gmp",
			";?\\s*extension=intl\\s*":       "extension=intl",
			";?\\s*extension=mbstring\\s*":   "extension=mbstring",
			";?\\s*extension=openssl\\s*":    "extension=openssl",
			";?\\s*extension=pdo_mysql\\s*":  "extension=pdo_mysql",
			";?\\s*extension=pdo_sqlite\\s*": "extension=pdo_sqlite",
			";?\\s*extension=sqlite\\s*":     "extension=sqlite3",
			";?\\s*extension=sockets\\s*":    "extension=sockets",
			";?\\s*extension=zip\\s*":        "extension=zip",
		})
		if err != nil {
			return errors.WithMessage(err, "failed to update extensions to php.ini")
		}

		cacertPath := filepath.Join(filepath.Dir(iniFilePath), "cacert.pem")

		err = utils.DownloadFile(ctx, caCertURL, cacertPath)
		if err != nil {
			return errors.WithMessage(err, "failed to download cacert.pem")
		}

		err = utils.FindLineAndReplaceOrAdd(ctx, iniFilePath, map[string]string{
			";?\\s*curl\\.cainfo\\s*":    fmt.Sprintf(`curl.cainfo="%s"`, cacertPath),
			";?\\s*openssl\\.cafile\\s*": fmt.Sprintf(`openssl.cafile="%s"`, cacertPath),
		})
		if err != nil {
			return errors.WithMessage(err, "failed to update cacert.pem path in php.ini")
		}

		return nil
	},
}

var packagePostProcessors = map[string]func(ctx context.Context, packagePath string) error{
	PHPPackage: func(_ context.Context, packagePath string) error {
		log.Printf("Adding %s to PATH", packagePath)

		path, _ := os.LookupEnv("PATH")

		currentPath := strings.Split(path, string(filepath.ListSeparator))
		if utils.Contains(currentPath, packagePath) {
			log.Println("Path already contains ", packagePath)
			log.Println("PATH: ", strings.Join(currentPath, string(filepath.ListSeparator)))

			return nil
		}

		newPath := append(currentPath, packagePath)

		log.Println("New PATH: ", strings.Join(newPath, string(filepath.ListSeparator)))

		err := os.Setenv("PATH", path+string(os.PathListSeparator)+packagePath)
		if err != nil {
			return errors.WithMessage(err, "failed to set PATH")
		}

		return nil
	},
	NginxPackage: func(_ context.Context, packagePath string) error {
		entries, err := os.ReadDir(packagePath)
		if err != nil {
			return err
		}

		if len(entries) != 1 {
			return NewErrInvalidDirContents(packagePath)
		}

		d := filepath.Join(packagePath, entries[0].Name())

		entries, err = os.ReadDir(d)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = utils.Move(filepath.Join(d, entry.Name()), filepath.Join(packagePath, entry.Name()))
			if err != nil {
				return errors.WithMessage(err, "failed to move file")
			}
		}

		log.Println("Removing", d)

		return os.RemoveAll(d)
	},
	ComposerPackage: func(_ context.Context, _ string) error {
		// Wait composer installation

		tries := 10
		for tries > 0 {
			for _, p := range repository[ComposerPackage].LookupPath {
				if _, err := exec.LookPath(p); err == nil {
					return nil
				}
			}
			tries--
		}

		return errors.New("failed to install composer, failed to lookup composer executable")
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

	Env []env `xml:"env,omitempty"`
}

type onFailure struct {
	Action string `xml:"action,attr"`
	Delay  string `xml:"delay,attr,omitempty"`
}

type env struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}
