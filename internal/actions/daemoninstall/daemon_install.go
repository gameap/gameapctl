package daemoninstall

import (
	"bytes"
	"container/heap"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
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
	"strings"
	"time"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
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
}

//nolint:funlen
func Handle(cliCtx *cli.Context) error {
	fmt.Println("Install daemon")
	state := daemonsInstallState{
		Host:         cliCtx.String("host"),
		Token:        cliCtx.String("token"),
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

	fmt.Println("Downloading runner ...")
	state, err = downloadRunner(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to download runner")
	}

	fmt.Println("Configuring gameap-daemon ...")
	state, err = configureDaemon(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to configure daemon")
	}

	state, err = saveDaemonConfig(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to save daemon config")
	}

	fmt.Println("Starting gameap-daemon ...")
	err = daemon.Start(cliCtx.Context)
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

//nolint:funlen
func generateCertificates(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return state, errors.WithMessage(err, "failed to get hostname")
	}

	if _, err := os.Stat(state.CertsPath); os.IsNotExist(err) {
		err = os.MkdirAll(state.CertsPath, 0700) //nolint:gomnd
		if err != nil {
			return state, errors.WithMessage(err, "failed to create certificates directory")
		}
	}

	var privKey *rsa.PrivateKey
	privKeyFilePath := filepath.Join(state.CertsPath, "server.key")

	_, err = os.Stat(privKeyFilePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		privKey, err = rsa.GenerateKey(rand.Reader, 2048) //nolint:gomnd
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
		err = pem.Encode(f, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKey),
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
			return state, errors.WithMessage(err, "failed to parse private key")
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

func installDaemonBinaries(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	tmpDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return state, errors.WithMessage(err, "failed to make temp dir")
	}

	daemonBinariesTmpDir := filepath.Join(tmpDir, "daemon")

	downloadURL, err := findReleaseURL(ctx, state.OSInfo.Kernel, state.OSInfo.Platform)
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

	return state, nil
}

func downloadRunner(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	runnerFilePath := filepath.Join(gameap.DefaultToolsPath, "runner.sh")

	err := utils.DownloadFile(
		ctx,
		"https://raw.githubusercontent.com/gameap/scripts/master/process-manager/tmux/runner.sh",
		runnerFilePath,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to download runner")
	}

	err = os.Chmod(runnerFilePath, 0755)
	if err != nil {
		return state, errors.WithMessage(err, "failed to chmod runner")
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
	_, _ = fw.Write([]byte("{node_work_path}/runner.sh get_console -d {dir} -n {uuid} -u {user}"))

	fw, _ = w.CreateFormField("script_get_console")
	_, _ = fw.Write([]byte("{node_work_path}/runner.sh send_command -d {dir} -n {uuid} -u {user} -c \"{command}\""))

	client := http.Client{
		Timeout: 30 * time.Second, //nolint:gomnd
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

	statusParts := bytes.SplitN(parts[0], []byte(" "), 3) //nolint:gomnd

	if string(statusParts[0]) != "Success" {
		dumpRequestAndResponse(requestClone, response)
		log.Println(string(result))

		if len(statusParts) > 1 {
			return state, UnableToSetupNodeError(bytes.Join(statusParts[1:], []byte(" ")))
		}

		return state, UnableToSetupNodeError("error, no message")
	}

	//nolint:gomnd
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
		Timeout: 5 * time.Second, //nolint:gomnd
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

		//nolint:gomnd
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

	Scripts DaemonConfigScripts
}

type releases struct {
	TagName string  `json:"tag_name"` //nolint:tagliatelle
	Assets  []asset `json:"assets"`
}

type asset struct {
	URL                string `json:"url"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"` //nolint:tagliatelle
}

func findReleaseURL(_ context.Context, kernel, platform string) (string, error) {
	resp, err := http.Get("https://api.github.com/repos/gameap/daemon/releases") //nolint:bodyclose,noctx
	if err != nil {
		return "", errors.WithMessage(err, "failed to get releases")
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp.Body)

	link, err := findRelease(resp.Body, strings.ToLower(kernel), strings.ToLower(platform))
	if err != nil {
		return "", errors.WithMessage(err, "failed to find release")
	}

	return link, nil
}

func findRelease(reader io.Reader, os string, arch string) (string, error) {
	r := []releases{}
	d := json.NewDecoder(reader)
	err := d.Decode(&r)
	if err != nil {
		return "", err
	}

	for _, release := range r {
		archiveName := fmt.Sprintf("gameap-daemon-%s-%s-%s.tar.gz", release.TagName, os, arch)
		archiveNameWindows := fmt.Sprintf("gameap-daemon-%s-%s-%s.zip", release.TagName, os, arch)

		for _, asset := range release.Assets {
			if asset.Name == archiveName {
				return asset.BrowserDownloadURL, nil
			}

			if os == "windows" && asset.Name == archiveNameWindows {
				return asset.BrowserDownloadURL, nil
			}
		}
	}

	return "", nil
}
