package daemoninstall

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
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

const (
	defaultDaemonPort = 31717
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
	Host  string
	Token string

	WorkPath     string
	SteamCMDPath string
	CertsPath    string

	OSInfo osinfo.Info

	ListenIP string
	NodeID   uint
	APIKey   string

	ProcessManager string

	User     string
	Password string // Password for user (Windows only)
}

func Handle(cliCtx *cli.Context) error {
	return Install(
		cliCtx.Context,
		cliCtx.String("host"),
		cliCtx.String("token"),
	)
}

//nolint:funlen,gocognit
func Install(ctx context.Context, host, token string) error {
	fmt.Println("Install daemon")

	state := daemonsInstallState{
		Host:         host,
		Token:        token,
		SteamCMDPath: gameap.DefaultSteamCMDPath,
	}

	if state.Host == "" {
		return errEmptyHost
	}

	if state.Token == "" {
		return errEmptyToken
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

		if state.ProcessManager == "tmux" {
			fmt.Println("Checking for tmux ...")
			if !utils.IsCommandAvailable("tmux") {
				fmt.Println("Installing tmux ...")
				if err = pm.Install(ctx, packagemanager.TmuxPackage); err != nil {
					return errors.WithMessage(err, "failed to install tmux")
				}
			}
		}
	}

	state, err = createUser(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to create user")
	}

	fmt.Println("Installing steamcmd ...")
	state, err = installSteamCMD(ctx, pm, state)
	if err != nil {
		return errors.WithMessage(err, "failed install SteamCMD")
	}

	if state.OSInfo.Distribution != packagemanager.DistributionWindows {
		if err = pm.Install(
			ctx,
			packagemanager.UnzipPackage,
			packagemanager.XZUtilsPackage,
		); err != nil {
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

	fmt.Println("Generating GameAP Daemon certificates ...")
	state, err = generateCertificates(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to generate certificates")
	}

	fmt.Println("Setting firewall rules ...")
	state, err = setFirewallRules(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to set firewall rules")
	}

	fmt.Println("Downloading gameap-daemon binaries ...")
	state, err = installDaemonBinaries(ctx, pm, state)
	if err != nil {
		return errors.WithMessage(err, "failed to install daemon binaries")
	}

	fmt.Println("Configuring gameap-daemon ...")
	state, err = configureDaemon(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to configure daemon")
	}

	state, err = saveDaemonConfig(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to save daemon config")
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
	if runtime.GOOS == "linux" {
		err := utils.Download(
			ctx,
			"https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz",
			state.SteamCMDPath,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to download steamcmd")
		}
	}

	if runtime.GOOS == "windows" {
		err := utils.Download(
			ctx,
			"https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip",
			state.SteamCMDPath,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to download steamcmd")
		}
	}

	if runtime.GOOS == "linux" && strconv.IntSize == 64 {
		fmt.Println("Installing 32-bit libraries ...")
		err := pm.Install(
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

//nolint:funlen,gocognit
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

	ips := detectIPs()
	state.ListenIP, err = chooseBestIP(ips)
	if err != nil {
		return state, errors.WithMessage(err, "failed to choose best IP")
	}

	ips = removeLocalIPs(ips)

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

	client := http.Client{
		Timeout: 30 * time.Second, //nolint:mnd
	}

	u, err := url.JoinPath(state.Host, "/gdaemon/create/", state.Token)
	if err != nil {
		return state, errors.WithMessage(err, "failed to create daemon create url")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &b)
	if err != nil {
		return state, errors.WithMessage(err, "failed to create daemon create request")
	}
	request.Header.Set("Content-Type", w.FormDataContentType())

	requestClone := request.Clone(ctx)

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

func dumpRequestAndResponse(req *http.Request, res *http.Response) {
	dumpRequest, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.Println(err)
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
		pw := base64.StdEncoding.EncodeToString([]byte(state.Password))
		cfg.Users = map[string]string{
			state.User: "base64:" + pw,
		}
	}

	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return state, errors.WithMessage(err, "failed to marshal daemon config")
	}

	err = os.WriteFile(gameap.DefaultDaemonConfigFilePath, cfgBytes, 0600)

	return state, err
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
		//nolint:bodyclose,noctx
		r, err := client.Get(d)
		if err != nil {
			continue
		}
		if r.StatusCode != http.StatusOK {
			continue
		}
		defer func(body io.ReadCloser) {
			err := body.Close()
			if err != nil {
				log.Println(errors.WithMessage(err, "failed to close response body"))
			}
		}(r.Body)

		//nolint:mnd
		if r.ContentLength > 20 {
			continue
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			continue
		}

		if len(b) == 0 {
			continue
		}

		return string(b)
	}

	return location
}

func detectIPs() []string {
	ips := make([]string, 0)

	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			switch v := a.(type) {
			case *net.IPNet:
				ips = append(ips, v.IP.String())
			case *net.IPAddr:
				ips = append(ips, v.IP.String())
			}
		}
	}

	return ips
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

func removeLocalIPs(ips []string) []string {
	result := make([]string, 0, len(ips))

	for _, ip := range ips {
		if utils.IsIPv4(ip) {
			if ip[:4] == "127." {
				continue
			}
		}

		if utils.IsIPv6(ip) {
			if ip == "::1" || ip[:2] == "fc" || ip[:2] == "fd" || ip[:2] == "fe" {
				continue
			}
		}

		result = append(result, ip)
	}

	return result
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
