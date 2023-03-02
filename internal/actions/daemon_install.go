package actions

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var errEmptyToken = errors.New("empty token")

type daemonsInstallState struct {
	Host  string
	Token string

	WorkPath     string
	SteamCMDPath string

	OSInfo osinfo.Info
}

func DaemonInstall(cliCtx *cli.Context) error {
	fmt.Println("Install daemon")
	state := daemonsInstallState{
		Host:         cliCtx.String("host"),
		Token:        cliCtx.String("token"),
		SteamCMDPath: defaultSteamCMDPath,
	}

	fmt.Printf("%+v \n", state)

	if state.Host == "" {
		return errEmptyHost
	}

	if state.Token == "" {
		return errEmptyToken
	}

	if state.WorkPath == "" {
		state.WorkPath = defaultWorkPath
	}

	state.OSInfo = contextInternal.OSInfoFromContext(cliCtx.Context)

	pm, err := packagemanager.Load(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	fmt.Println("Checking for updates ...")
	if err = pm.CheckForUpdates(cliCtx.Context); err != nil {
		return errors.WithMessage(err, "failed to check for updates")
	}

	fmt.Println("Checking for curl ...")
	if !utils.IsCommandAvailable("curl") {
		fmt.Println("Installing curl ...")
		if err = pm.Install(cliCtx.Context, packagemanager.CurlPackage); err != nil {
			return errors.WithMessage(err, "failed to install curl")
		}
	}

	fmt.Println("Checking for gpg ...")
	if !utils.IsCommandAvailable("gpg") {
		fmt.Println("Installing gpg ...")
		if err = pm.Install(cliCtx.Context, packagemanager.GnuPGPackage); err != nil {
			return errors.WithMessage(err, "failed to install gpg")
		}
	}

	state, err = createUser(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to create user")
	}

	fmt.Println("Installing steamcmd ...")
	state, err = installSteamCMD(cliCtx.Context, pm, state)
	if err != nil {
		return errors.WithMessage(err, "failed install SteamCMD")
	}

	if err = pm.Install(
		cliCtx.Context,
		packagemanager.UnzipPackage,
		packagemanager.XZUtilsPackage,
	); err != nil {
		return errors.WithMessage(err, "failed to install archive managers")
	}

	fmt.Println("Generating GameAP Daemon certificates ...")
	state, err = generateCertificates(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to generate certificates")
	}

	fmt.Println("Downloading gameap-daemon binaries ...")
	state, err = installDaemonBinaries(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to install daemon binaries")
	}

	return nil
}

func installSteamCMD(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state daemonsInstallState,
) (daemonsInstallState, error) {
	err := utils.Download(
		ctx,
		"https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz",
		state.SteamCMDPath,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to download steamcmd")
	}

	if runtime.GOOS == "linux" && strconv.IntSize == 64 {
		fmt.Println("Installing 32-bit libraries ...")
		err = pm.Install(
			ctx,
			packagemanager.Lib32GCCPackage,
			packagemanager.Lib32Stdc6Package,
			packagemanager.Lib32z1Package,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to install 32 bit libraries")
		}
	}

	return state, nil
}

func generateCertificates(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return state, errors.WithMessage(err, "failed to get hostname")
	}

	var privKey []byte
	privKeyFilePath := filepath.Join(state.WorkPath, "daemon", "certs", "server.key")

	if _, err = os.Stat(privKeyFilePath); errors.Is(err, fs.ErrNotExist) {
		_, privKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return state, errors.WithMessage(err, "failed to generate key")
		}

		err = os.WriteFile(
			filepath.Join(state.WorkPath, "daemon", "certs", "server.key"),
			privKey,
			0600,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to save private key")
		}
	} else if err != nil {
		return state, errors.WithMessage(err, "failed to stat private key file")
	} else {
		fmt.Println("Private key is already exists ...")

		privKey, err = os.ReadFile(privKeyFilePath)
		if err != nil {
			return state, errors.WithMessage(err, "failed to read private key file")
		}
	}

	var csrBytes []byte
	csrFilePath := filepath.Join(state.WorkPath, "daemon", "certs", "server.csr")

	if _, err = os.Stat(csrFilePath); errors.Is(err, fs.ErrNotExist) {
		csr := &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName:   hostname,
				Organization: []string{"GameAP Daemon"},
			},
		}

		csrBytes, err = x509.CreateCertificateRequest(rand.Reader, csr, privKey)
		if err != nil {
			return state, errors.WithMessage(err, "failed to create certificate request")
		}

		err = os.WriteFile(
			csrFilePath,
			csrBytes,
			0600,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to save certificate request")
		}
	} else if err != nil {
		return state, errors.WithMessage(err, "failed to stat certificate request file")
	} else {
		fmt.Println("Certificate request is already exists ...")
	}

	return state, nil
}

func installDaemonBinaries(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	tmpDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return state, errors.WithMessage(err, "failed to make temp dir")
	}

	daemonBinariesTmpDir := filepath.Join(tmpDir, "daemon")

	utils.Download(
		ctx,
		"https://packages.gameap.ru/gameap-daemon/download-release?os=linux&arch=$(arch)",
		daemonBinariesTmpDir,
	)

	var binariesInstalled bool

	for _, p := range []string{"gameap-daemon", "gameap-daemon.exe"} {
		fp := filepath.Join(daemonBinariesTmpDir, p)
		if _, err = os.Stat(fp); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return state, errors.WithMessage(err, "failed to stat file")
		}

		err = utils.Move(fp, defaultDaemonFilePath)
		if err != nil {
			return state, errors.WithMessage(err, "failed to move gameap-daemon binaries")
		}
		binariesInstalled = true
	}

	if !binariesInstalled {
		return state, errors.New("gameap binaries wasn't installed, invalid archive contents")
	}

	return state, nil
}

type DaemonConfigScripts struct {
	Install     string
	Reinstall   string
	Update      string
	Start       string
	Pause       string
	Unpause     string
	Stop        string
	Kill        string
	Restart     string
	Status      string
	GetConsole  string
	SendCommand string
	Delete      string
}

type DaemonSteamConfig struct {
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
}

type DaemonConfig struct {
	NodeID uint `yaml:"ds_id"`

	ListenIP   string `yaml:"listen_ip"`
	ListenPort int    `yaml:"listen_port"`

	APIHost string `yaml:"api_host"`
	APIKey  string `yaml:"api_key"`

	DaemonLogin            string `yaml:"daemon_login"`
	DaemonPassword         string `yaml:"daemon_password"`
	PasswordAuthentication bool   `yaml:"password_authentication"`

	CACertificateFile    string `yaml:"ca_certificate_file"`
	CertificateChainFile string `yaml:"certificate_chain_file"`
	PrivateKeyFile       string `yaml:"private_key_file"`
	PrivateKeyPassword   string `yaml:"private_key_password"`
	DHFile               string `yaml:"dh_file"`

	IFList     []string `yaml:"if_list"`
	DrivesList []string `yaml:"drives_list"`

	StatsUpdatePeriod   int `yaml:"stats_update_period"`
	StatsDBUpdatePeriod int `yaml:"stats_db_update_period"`

	// Log config
	LogLevel  string `yaml:"log_level"`
	OutputLog string `yaml:"output_log"`
	ErrorLog  string `yaml:"error_log"`

	// Dedicated server config
	Path7zip    string `yaml:"path_7zip"`
	PathStarter string `yaml:"path_starter"`

	WorkPath     string `yaml:"work_path"`
	ToolsPath    string `yaml:"tools_path"`
	SteamCMDPath string `yaml:"steamcmd_path"`

	SteamConfig DaemonSteamConfig `yaml:"steam_config"`

	Scripts DaemonConfigScripts
}
