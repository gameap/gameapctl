package install

import (
	"bytes"
	"container/heap"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	daemonpkg "github.com/gameap/gameapctl/internal/pkg/daemon"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	defaultDaemonPort = 31717
)

const (
	processManagerDefault = processManagerTmux
	processManagerSystemD = "systemd"
	processManagerDocker  = "docker"
	processManagerPodman  = "podman"
	processManagerSimple  = "simple"
	processManagerTmux    = "tmux"
)

var errEmptyToken = errors.New("empty token")

type UnableToSetupNodeError string

func (e UnableToSetupNodeError) Error() string {
	return "unable to setup node: " + string(e)
}

type InvalidResponseStatusCodeError int

func (e InvalidResponseStatusCodeError) Error() string {
	return "invalid response status code " + strconv.Itoa(int(e))
}

type daemonsInstallState struct {
	Host       string
	Token      string
	ConnectURL string
	Config     string

	WorkPath     string
	SteamCMDPath string
	CertsPath    string

	OSInfo osinfo.Info

	ListenIP string
	NodeID   uint
	APIKey   string

	ProcessManager string

	FromGithub bool
	Branch     string

	User     string
	Password string // Password for user (Windows only)
}

type InstallOptions struct {
	Host       string
	Token      string
	ConnectURL string
	Config     string
	FromGithub bool
	Branch     string
}

func Handle(cliCtx *cli.Context) error {
	return Install(cliCtx.Context, InstallOptions{
		Host:       cliCtx.String("host"),
		Token:      cliCtx.String("token"),
		ConnectURL: cliCtx.String("connect"),
		Config:     cliCtx.String("config"),
		FromGithub: cliCtx.Bool("github"),
		Branch:     cliCtx.String("branch"),
	})
}

//nolint:gocognit,funlen,gocyclo
func Install(ctx context.Context, opts InstallOptions) error {
	fmt.Println("Install daemon")

	if opts.Branch == "" {
		opts.Branch = "master"
	}

	if opts.ConnectURL != "" && (opts.Host != "" || opts.Token != "") {
		return errors.New("--connect and --host/--token are mutually exclusive")
	}

	if opts.ConnectURL != "" {
		if _, err := ParseConnectURL(opts.ConnectURL); err != nil {
			return errors.WithMessage(err, "invalid connect URL")
		}
	}

	if opts.ConnectURL == "" && opts.Host == "" {
		return errEmptyHost
	}

	if opts.ConnectURL == "" && opts.Token == "" {
		return errEmptyToken
	}

	state := daemonsInstallState{
		Host:         opts.Host,
		Token:        opts.Token,
		ConnectURL:   opts.ConnectURL,
		Config:       opts.Config,
		FromGithub:   opts.FromGithub,
		Branch:       opts.Branch,
		SteamCMDPath: gameap.DefaultSteamCMDPath,
	}

	if state.WorkPath == "" {
		state.WorkPath = gameap.DefaultWorkPath
	}

	if state.CertsPath == "" {
		state.CertsPath = gameap.DefaultDaemonCertPath
	}

	state.OSInfo = contextInternal.OSInfoFromContext(ctx)

	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	fmt.Println("Checking for updates ...")
	if err = pm.CheckForUpdates(ctx); err != nil {
		return errors.WithMessage(err, "failed to check for updates")
	}

	fmt.Println("Defining process manager ...")
	state, err = defineProcessManager(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to configure process manager")
	}

	//nolint:nestif
	if state.OSInfo.Distribution != packagemanager.DistributionWindows {
		fmt.Println("Checking for curl ...")
		if !utils.IsCommandAvailable("curl") {
			fmt.Println("Installing curl ...")
			if err = pm.Install(ctx, packagemanager.CurlPackage); err != nil {
				return errors.WithMessage(err, "failed to install curl")
			}
		}

		fmt.Println("Checking for gpg ...")
		if !utils.IsCommandAvailable("gpg") {
			fmt.Println("Installing gpg ...")
			if err = pm.Install(ctx, packagemanager.GnuPGPackage); err != nil {
				return errors.WithMessage(err, "failed to install gpg")
			}
		}

		if state.ProcessManager == processManagerDefault {
			fmt.Println("Checking for tmux ...")

			if !utils.IsCommandAvailable("tmux") {
				fmt.Println("Installing tmux ...")
				if err = pm.Install(ctx, packagemanager.TmuxPackage); err != nil {
					return errors.WithMessage(err, "failed to install tmux")
				}
			}
		}

		if state.ProcessManager == processManagerDocker {
			fmt.Println("Checking for docker ...")

			if !utils.IsCommandAvailable("docker") {
				fmt.Println("Installing docker ...")

				if err = pm.Install(ctx, packagemanager.DockerPackage); err != nil {
					return errors.WithMessage(err, "failed to install docker")
				}
			}
		}
	}

	state, err = createUser(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to create user")
	}

	if !utils.IsFileExists(filepath.Join(state.WorkPath, "servers")) {
		fmt.Println("Creating servers directory ...")

		err = os.MkdirAll(filepath.Join(state.WorkPath, "servers"), 0755)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to create servers directory %s",
				filepath.Join(state.WorkPath, "servers"),
			)
		}
	}

	if state.OSInfo.Platform.IsX86() {
		fmt.Println("Installing steamcmd ...")
		state, err = installSteamCMD(ctx, pm, state)
		if err != nil {
			return errors.WithMessage(err, "failed install SteamCMD")
		}
	}

	if state.OSInfo.Distribution != packagemanager.DistributionWindows {
		if err = pm.Install(ctx, packagemanager.UnzipPackage); err != nil {
			return errors.WithMessage(err, "failed to install archive managers")
		}
		if err = pm.Install(ctx, packagemanager.XZUtilsPackage); err != nil {
			return errors.WithMessage(err, "failed to install archive managers")
		}
	}

	fmt.Println("Installing gameap-daemon dependencies ...")
	state, err = installOSSpecificPackages(ctx, pm, state)
	if err != nil {
		return errors.WithMessage(err, "failed to set user privileges")
	}

	fmt.Println("Set user privileges ...")
	state, err = setUserPrivileges(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to set user privileges")
	}

	fmt.Println("Setting firewall rules ...")
	state, err = setFirewallRules(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to set firewall rules")
	}

	if state.FromGithub {
		fmt.Println("Building gameap-daemon from GitHub source ...")
		state, err = installDaemonFromGithub(ctx, pm, state)
	} else {
		fmt.Println("Downloading gameap-daemon binaries ...")
		state, err = installDaemonBinaries(ctx, pm, state)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to install daemon binaries")
	}

	if state.ConnectURL != "" {
		state, err = enrollFlow(ctx, state)
	} else {
		state, err = legacyConfigureFlow(ctx, state)
	}
	if err != nil {
		return err
	}

	if saveErr := gameapctl.SaveDaemonInstallState(ctx, gameapctl.DaemonInstallState{
		Host:           state.Host,
		ConnectURL:     state.ConnectURL,
		WorkPath:       state.WorkPath,
		SteamCMDPath:   state.SteamCMDPath,
		CertsPath:      state.CertsPath,
		FromGithub:     state.FromGithub,
		Branch:         state.Branch,
		ProcessManager: state.ProcessManager,
	}); saveErr != nil {
		log.Println("Warning: failed to save daemon install state:", saveErr)
	}

	fmt.Println("Starting gameap-daemon ...")
	err = daemon.Start(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to start gameap-daemon")
	}

	return nil
}

func installSteamCMD(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state daemonsInstallState,
) (daemonsInstallState, error) {
	steamcmdBinary := filepath.Join(state.SteamCMDPath, "steamcmd.sh")
	if runtime.GOOS == "windows" {
		steamcmdBinary = filepath.Join(state.SteamCMDPath, "steamcmd.exe")
	}

	if !utils.IsFileExists(steamcmdBinary) {
		switch runtime.GOOS {
		case "linux":
			err := utils.Download(
				ctx,
				"https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz",
				state.SteamCMDPath,
			)
			if err != nil {
				return state, errors.WithMessage(err, "failed to download steamcmd")
			}
		case "windows":
			err := utils.Download(
				ctx,
				"https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip",
				state.SteamCMDPath,
			)
			if err != nil {
				return state, errors.WithMessage(err, "failed to download steamcmd")
			}
		}
	} else {
		fmt.Println("SteamCMD already installed, skipping download ...")
	}

	if runtime.GOOS == "linux" && strconv.IntSize == 64 {
		fmt.Println("Installing 32-bit libraries ...")
		err := pm.Install(ctx, packagemanager.Lib32GCCPackage)
		if err != nil {
			return state, errors.WithMessage(err, "failed to install 32 bit libraries")
		}
		err = pm.Install(ctx, packagemanager.Lib32Stdc6Package)
		if err != nil {
			return state, errors.WithMessage(err, "failed to install 32 bit libraries")
		}
		err = pm.Install(ctx, packagemanager.Lib32z1Package)
		if err != nil {
			return state, errors.WithMessage(err, "failed to install 32 bit libraries")
		}
	}

	return state, nil
}

//nolint:gocognit,funlen
func generateCertificates(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return state, errors.WithMessage(err, "failed to get hostname")
	}

	if _, err := os.Stat(state.CertsPath); os.IsNotExist(err) {
		err = os.MkdirAll(state.CertsPath, 0700) //nolint:mnd
		if err != nil {
			return state, errors.WithMessage(err, "failed to create certificates directory")
		}
	}

	var privKey any
	privKeyFilePath := filepath.Join(state.CertsPath, "server.key")

	_, err = os.Stat(privKeyFilePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		privKeyRsa, err := rsa.GenerateKey(rand.Reader, 2048) //nolint:mnd
		if err != nil {
			return state, errors.WithMessage(err, "failed to generate key")
		}

		f, err := os.OpenFile(privKeyFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return state, errors.WithMessage(err, "failed to create private key file")
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				log.Println(errors.WithMessage(err, "failed to close private key file"))
			}
		}(f)

		privKey = privKeyRsa

		err = pem.Encode(f, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKeyRsa),
		})
		if err != nil {
			return state, errors.WithMessage(err, "failed to encode private key")
		}
	case err != nil:
		return state, errors.WithMessage(err, "failed to stat private key file")
	default:
		fmt.Println("Private key is already exists ...")

		b, err := os.ReadFile(privKeyFilePath)
		if err != nil {
			return state, errors.WithMessage(err, "failed to read private key file")
		}
		block, _ := pem.Decode(b)
		if block == nil {
			return state, errors.New("failed to decode private key")
		}

		privKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to parse private key"))
			privKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return state, errors.WithMessage(err, "failed to parse private key")
			}
		}
	}

	var csrBytes []byte
	csrFilePath := filepath.Join(state.CertsPath, "server.csr")

	_, err = os.Stat(csrFilePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
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

		f, err := os.OpenFile(csrFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return state, errors.WithMessage(err, "failed to create certificate request file")
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				log.Println(errors.WithMessage(err, "failed to close certificate request file"))
			}
		}(f)

		err = pem.Encode(f, &pem.Block{
			Type:  "CERTIFICATE REQUEST",
			Bytes: csrBytes,
		})
		if err != nil {
			return state, errors.WithMessage(err, "failed to encode certificate request")
		}
	case err != nil:
		return state, errors.WithMessage(err, "failed to stat certificate request file")
	default:
		fmt.Println("Certificate request is already exists ...")
	}

	return state, nil
}

func installDaemonBinaries(
	ctx context.Context, pm packagemanager.PackageManager, state daemonsInstallState,
) (daemonsInstallState, error) {
	tmpDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return state, errors.WithMessage(err, "failed to make temp dir")
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			log.Printf("Failed to remove temp dir %s: %v\n", tmpDir, removeErr)
		}
	}()

	daemonBinariesTmpDir := filepath.Join(tmpDir, "daemon")

	downloadURL, err := findDaemonReleaseURL(ctx)
	if err != nil {
		return state, errors.WithMessage(err, "failed to find release")
	}

	err = utils.Download(
		ctx,
		downloadURL,
		daemonBinariesTmpDir,
	)
	if err != nil {
		log.Println("Download url: ", downloadURL)

		return state, errors.WithMessage(err, "failed to download gameap-daemon binaries")
	}

	var binariesInstalled bool

	for _, p := range []string{"gameap-daemon", "gameap-daemon.exe"} {
		fp := filepath.Join(daemonBinariesTmpDir, p)
		if _, err = os.Stat(fp); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return state, errors.WithMessage(err, "failed to stat file")
		}

		err = utils.Move(fp, gameap.DefaultDaemonFilePath)
		if err != nil {
			return state, errors.WithMessage(err, "failed to move gameap-daemon binaries")
		}
		binariesInstalled = true

		break
	}

	if !binariesInstalled {
		return state, errors.New("gameap binaries wasn't installed, invalid archive contents")
	}

	if state.OSInfo.Distribution == packagemanager.DistributionWindows {
		err = pm.Install(ctx, packagemanager.GameAPDaemon)
		if err != nil {
			return state, errors.WithMessage(err, "failed to install gameap-daemon")
		}
	}

	return state, nil
}

func installDaemonFromGithub(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state daemonsInstallState,
) (daemonsInstallState, error) {
	if err := daemonpkg.SetupDaemonFromGithub(ctx, pm, state.Branch); err != nil {
		return state, errors.WithMessage(err, "failed to build daemon from github")
	}

	return state, nil
}

//nolint:funlen
func configureDaemon(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return state, errors.WithMessage(err, "failed to get hostname")
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormField("name")
	_, _ = fw.Write([]byte(hostname))
	fw, _ = w.CreateFormField("location")
	_, _ = fw.Write([]byte(detectLocation()))
	fw, _ = w.CreateFormField("work_path")
	_, _ = fw.Write([]byte(state.WorkPath))
	fw, _ = w.CreateFormField("steamcmd_path")
	_, _ = fw.Write([]byte(state.SteamCMDPath))
	fw, _ = w.CreateFormField("os")
	_, _ = fw.Write([]byte(runtime.GOOS))

	ips := utils.DetectIPs()
	state.ListenIP, err = chooseBestIP(ips)
	if err != nil {
		return state, errors.WithMessage(err, "failed to choose best IP")
	}

	ips = utils.RemoveLocalIPs(ips)

	for _, ip := range ips {
		fw, _ = w.CreateFormField("ip[]")
		_, _ = fw.Write([]byte(ip))
	}

	fw, _ = w.CreateFormField("gdaemon_host")
	_, _ = fw.Write([]byte(state.ListenIP))

	fw, _ = w.CreateFormField("gdaemon_port")
	_, _ = fw.Write([]byte("31717"))

	csrFilePath := filepath.Join(state.CertsPath, "server.csr")
	csrBites, err := os.Open(csrFilePath)
	if err != nil {
		return state, errors.WithMessage(err, "failed to open certificate request file")
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to close certificate request file"))
		}
	}(csrBites)

	fw, err = w.CreateFormFile("gdaemon_server_cert", "gdaemon_server_cert")
	if err != nil {
		return state, errors.WithMessage(err, "failed to create form file")
	}

	_, err = io.Copy(fw, csrBites)
	if err != nil {
		return state, errors.WithMessage(err, "failed to write certificate request file contents to form")
	}

	fw, _ = w.CreateFormField("script_get_console")
	_, _ = fw.Write([]byte("server-output {id}"))

	fw, _ = w.CreateFormField("script_send_command")
	_, _ = fw.Write([]byte("server-command {id} {command}"))

	err = w.Close()
	if err != nil {
		return state, errors.WithMessage(err, "failed to close multipart writer")
	}

	client := http.Client{
		Timeout: 30 * time.Second, //nolint:mnd
	}

	u, err := url.JoinPath(state.Host, "/gdaemon/create/", state.Token)
	if err != nil {
		return state, errors.WithMessage(err, "failed to create daemon create url")
	}

	// Read the buffer into a byte slice so we can create multiple readers
	bodyBytes := b.Bytes()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(bodyBytes))
	if err != nil {
		return state, errors.WithMessage(err, "failed to create daemon create request")
	}
	request.Header.Set("Content-Type", w.FormDataContentType())
	// Set GetBody to allow the body to be read multiple times (for retries and debugging)
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	requestClone := request.Clone(ctx)
	// Explicitly set the clone's body to a fresh reader to ensure it's not consumed
	requestClone.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	//nolint:bodyclose
	response, err := client.Do(request)
	if err != nil {
		return state, errors.WithMessage(err, "failed to post daemon create request")
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to close response body"))
		}
	}(response.Body)
	result, err := io.ReadAll(response.Body)
	if err != nil {
		return state, errors.WithMessage(err, "failed to read response body")
	}
	response.Body = io.NopCloser(bytes.NewBuffer(result))

	if response.StatusCode != http.StatusOK {
		err = InvalidResponseStatusCodeError(response.StatusCode)
		log.Println(err)

		dumpRequestAndResponse(requestClone, response)
		log.Println(string(result))

		return state, err
	}

	parts := bytes.SplitN(result, []byte("\n"), 2)
	if len(parts) == 0 {
		return state, errors.New("invalid response body")
	}

	statusParts := bytes.SplitN(parts[0], []byte(" "), 3) //nolint:mnd

	if string(statusParts[0]) != "Success" {
		dumpRequestAndResponse(requestClone, response)
		log.Println(string(result))

		if len(statusParts) > 1 {
			return state, UnableToSetupNodeError(bytes.Join(statusParts[1:], []byte(" ")))
		}

		return state, UnableToSetupNodeError("error, no message")
	}

	//nolint:mnd
	if len(statusParts) < 3 {
		return state, UnableToSetupNodeError("error, invalid status message")
	}

	nodeID, err := strconv.Atoi(string(statusParts[1]))
	if err != nil {
		return state, errors.WithMessage(err, "failed to convert node id to int")
	}
	if nodeID < 0 {
		return state, errors.New("node id cannot be negative")
	}

	state.NodeID = uint(nodeID)
	state.APIKey = string(statusParts[2])

	if len(parts) < 2 {
		return state, UnableToSetupNodeError("error, invalid body")
	}

	certificates := bytes.SplitN(parts[1], []byte("\n\n"), 2)
	if len(certificates) != 2 {
		return state, UnableToSetupNodeError("error, invalid certificates")
	}

	err = os.WriteFile(filepath.Join(state.CertsPath, "ca.crt"), certificates[0], 0600)
	if err != nil {
		return state, errors.WithMessage(err, "failed to write ca certificate")
	}

	err = os.WriteFile(filepath.Join(state.CertsPath, "server.crt"), certificates[1], 0600)
	if err != nil {
		return state, errors.WithMessage(err, "failed to write server certificate")
	}

	return state, nil
}

func enrollFlow(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	if _, statErr := os.Stat(state.CertsPath); os.IsNotExist(statErr) {
		if mkErr := os.MkdirAll(state.CertsPath, 0700); mkErr != nil { //nolint:mnd
			return state, errors.WithMessage(mkErr, "failed to create certificates directory")
		}
	}

	fmt.Println("Enrolling daemon via connect URL ...")

	if err := enrollDaemon(ctx, state); err != nil {
		return state, errors.WithMessage(err, "failed to enroll daemon via connect URL")
	}

	return state, nil
}

func legacyConfigureFlow(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	var err error

	fmt.Println("Generating GameAP Daemon certificates ...")
	state, err = generateCertificates(ctx, state)
	if err != nil {
		return state, errors.WithMessage(err, "failed to generate certificates")
	}

	fmt.Println("Configuring gameap-daemon ...")
	state, err = configureDaemon(ctx, state)
	if err != nil {
		return state, errors.WithMessage(err, "failed to configure daemon")
	}

	state, err = saveDaemonConfig(ctx, state)
	if err != nil {
		return state, errors.WithMessage(err, "failed to save daemon config")
	}

	return state, nil
}

func enrollDaemon(ctx context.Context, state daemonsInstallState) error {
	daemonPath := gameap.DefaultDaemonFilePath

	if _, err := os.Stat(daemonPath); os.IsNotExist(err) {
		return errors.New("gameap-daemon binary not found at " + daemonPath)
	}

	enrollCtx, cancel := context.WithTimeout(ctx, 60*time.Second) //nolint:mnd
	defer cancel()

	cmd := exec.CommandContext(enrollCtx, daemonPath, "enroll",
		"--connect="+state.ConnectURL,
		"--config-path="+gameap.DefaultDaemonConfigFilePath,
		"--certs-dir="+state.CertsPath,
		"--work-path="+state.WorkPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println(cmd.String())

	if err := cmd.Run(); err != nil {
		return errors.WithMessage(err, "gameap-daemon enroll failed")
	}

	return nil
}

func dumpRequestAndResponse(req *http.Request, res *http.Response) {
	dumpRequest, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to dump request"))
	} else {
		log.Println("Request:\n", string(dumpRequest))
	}

	dumpResponse, err := httputil.DumpResponse(res, true)
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Result: \n", string(dumpResponse))
	}
}

func saveDaemonConfig(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	cfg := DaemonConfig{
		NodeID: state.NodeID,

		ListenIP:   state.ListenIP,
		ListenPort: defaultDaemonPort,

		APIHost: state.Host,
		APIKey:  state.APIKey,

		CACertificateFile:    filepath.Join(state.CertsPath, "ca.crt"),
		CertificateChainFile: filepath.Join(state.CertsPath, "server.crt"),
		PrivateKeyFile:       filepath.Join(state.CertsPath, "server.key"),

		LogLevel:  "debug",
		OutputLog: gameap.DefaultOutputLogPath,

		WorkPath:     state.WorkPath,
		ToolsPath:    gameap.DefaultToolsPath,
		SteamCMDPath: state.SteamCMDPath,

		ProcessManager: ProcessManagerConfig{
			Name: state.ProcessManager,
		},
	}

	if state.OSInfo.Distribution == packagemanager.DistributionWindows {
		if state.User != "" {
			pw := base64.StdEncoding.EncodeToString([]byte(state.Password))

			cfg.Users = map[string]string{
				state.User: "base64:" + pw,
			}
		}

		cfg.UseNetworkServiceUser = true
	}

	if state.Config != "" {
		overrides := parseConfigOverrides(state.Config)
		applyConfigOverrides(&cfg, overrides)
	}

	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return state, errors.WithMessage(err, "failed to marshal daemon config")
	}

	err = os.WriteFile(gameap.DefaultDaemonConfigFilePath, cfgBytes, 0600)

	return state, err
}

func parseConfigOverrides(configEnv string) map[string]string {
	if configEnv == "" {
		return make(map[string]string)
	}

	configStr := configEnv

	decoded, err := base64.StdEncoding.DecodeString(configEnv)
	if err == nil && strings.Contains(string(decoded), "=") {
		configStr = string(decoded)
	}

	overrides := make(map[string]string)
	pairs := strings.Split(configStr, ";")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			log.Printf("Warning: skipping malformed config override: %s", pair)

			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		overrides[key] = value
	}

	return overrides
}

func applyConfigOverrides(cfg *DaemonConfig, overrides map[string]string) {
	for key, value := range overrides {
		switch {
		case key == "process_manager.name":
			fmt.Printf("Applying config override: %s=%s\n", key, value)
			cfg.ProcessManager.Name = value

		case strings.HasPrefix(key, "process_manager.config."):
			configKey := strings.TrimPrefix(key, "process_manager.config.")
			fmt.Printf("Applying config override: %s=%s\n", key, value)
			if cfg.ProcessManager.Config == nil {
				cfg.ProcessManager.Config = make(map[string]string)
			}
			cfg.ProcessManager.Config[configKey] = value

		case key == "listen_ip":
			fmt.Printf("Applying config override: %s=%s\n", key, value)
			cfg.ListenIP = value

		case key == "listen_port":
			port, err := strconv.Atoi(value)
			if err != nil {
				log.Printf("Warning: invalid listen_port value '%s', skipping", value)

				continue
			}
			fmt.Printf("Applying config override: %s=%s\n", key, value)
			cfg.ListenPort = port

		case key == "log_level":
			fmt.Printf("Applying config override: %s=%s\n", key, value)
			cfg.LogLevel = value

		case key == "work_path":
			fmt.Printf("Applying config override: %s=%s\n", key, value)
			cfg.WorkPath = value

		case key == "steamcmd_path":
			fmt.Printf("Applying config override: %s=%s\n", key, value)
			cfg.SteamCMDPath = value

		default:
			log.Printf("Warning: unknown config override key '%s', skipping", key)
		}
	}
}

func detectLocation() string {
	location := "unknown"

	detectors := []string{
		"https://ifconfig.co/country",
		"https://ifconfig.es/geo/country",
		"https://ipconfig.pw/country",
	}

	client := http.Client{
		Timeout: 5 * time.Second, //nolint:mnd
	}

	for _, d := range detectors {
		//nolint:noctx
		r, err := client.Get(d)
		if err != nil {
			continue
		}

		b, err := func() ([]byte, error) {
			defer r.Body.Close()

			if r.StatusCode != http.StatusOK {
				return nil, nil
			}

			//nolint:mnd
			if r.ContentLength > 20 {
				return nil, nil
			}

			return io.ReadAll(r.Body)
		}()
		if err != nil || len(b) == 0 {
			continue
		}

		return string(b)
	}

	return location
}

type WeightStruct struct {
	V any
	W int
}

type WeightStructHeap []WeightStruct

func (h WeightStructHeap) Len() int           { return len(h) }
func (h WeightStructHeap) Less(i, j int) bool { return h[i].W > h[j].W }
func (h WeightStructHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

//nolint:forcetypeassert
func (h *WeightStructHeap) Push(x interface{}) {
	*h = append(*h, x.(WeightStruct))
}

func (h *WeightStructHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]

	return x
}

func chooseBestIP(ips []string) (string, error) {
	if len(ips) == 0 {
		return "0.0.0.0", nil
	}

	h := make(WeightStructHeap, 0, len(ips))
	heap.Init(&h)

	for _, ip := range ips {
		switch {
		case utils.IsIPv6(ip):
			heap.Push(&h, WeightStruct{ip, 0})
		case ip[:4] == "127.":
			heap.Push(&h, WeightStruct{ip, 100})
		case ip[:4] == "169.":
			heap.Push(&h, WeightStruct{ip, 200})
		case ip[:4] == "172.":
			heap.Push(&h, WeightStruct{ip, 300})
		case ip[:3] == "10.":
			heap.Push(&h, WeightStruct{ip, 400})
		case ip[:7] == "192.168":
			heap.Push(&h, WeightStruct{ip, 500})
		default:
			heap.Push(&h, WeightStruct{ip, 1000})
		}
	}

	p, ok := heap.Pop(&h).(WeightStruct)
	if !ok {
		return "", errors.New("invalid pop struct")
	}

	result, ok := p.V.(string)
	if !ok {
		return "", errors.New("invalid value struct")
	}

	return result, nil
}

type DaemonConfigScripts struct {
	Install     string `yaml:"install,omitempty"`
	Reinstall   string `yaml:"reinstall,omitempty"`
	Update      string `yaml:"update,omitempty"`
	Start       string `yaml:"start,omitempty"`
	Pause       string `yaml:"pause,omitempty"`
	Unpause     string `yaml:"unpause,omitempty"`
	Stop        string `yaml:"stop,omitempty"`
	Kill        string `yaml:"kill,omitempty"`
	Restart     string `yaml:"restart,omitempty"`
	Status      string `yaml:"status,omitempty"`
	GetConsole  string `yaml:"get_console,omitempty"`  //nolint:tagliatelle
	SendCommand string `yaml:"send_command,omitempty"` //nolint:tagliatelle
	Delete      string `yaml:"delete,omitempty"`
}

type DaemonSteamConfig struct {
	Login    string `yaml:"login,omitempty"`
	Password string `yaml:"password,omitempty"`
}

type ProcessManagerConfig struct {
	Name   string            `yaml:"name,omitempty"`
	Config map[string]string `yaml:"config,omitempty"`
}

//nolint:tagliatelle
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

	Scripts DaemonConfigScripts `yaml:"-"`

	ProcessManager ProcessManagerConfig `yaml:"process_manager,omitempty"`

	Users map[string]string `yaml:"users"`

	// Windows specific settings

	// If true, the daemon will run servers under the "NT AUTHORITY\NETWORK SERVICE" user.
	// This user has limited permissions and is suitable for running game servers securely.
	// If false, servers will run under the user specified in the "users" section of the config.
	UseNetworkServiceUser bool `yaml:"use_network_service_user"`
}

func findDaemonReleaseURL(ctx context.Context) (string, error) {
	release, err := releasefinder.Find(
		ctx,
		"https://api.github.com/repos/gameap/daemon/releases",
		runtime.GOOS,
		runtime.GOARCH,
	)
	if err != nil {
		return "", errors.WithMessage(err, "failed to find release")
	}

	return release.URL, nil
}
