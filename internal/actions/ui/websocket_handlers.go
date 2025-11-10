package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	serviceStatusActive   = "active"
	serviceStatusInactive = "inactive"
	serviceStatusNotFound = "not found"
)

func cmdHandle(ctx context.Context, w io.Writer, m message) error {
	cmd := strings.Split(m.Value, " ")
	if len(cmd) == 0 {
		return errors.New("empty command")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var args []string
	if len(cmd) > 1 {
		args = cmd[1:]
	}

	switch cmd[0] {
	case "node-info":
		return nodeInfo(ctx, w, args)
	case "service-status":
		return serviceStatus(ctx, w, args)
	case "gameap-install":
		duplicateLogWriter(ctx, w)

		return gameapInstall(ctx, w, args)
	case "gameap-uninstall":
		duplicateLogWriter(ctx, w)

		return gameapUninstall(ctx, w, args)
	case "gameap-upgrade":
		duplicateLogWriter(ctx, w)

		return gameapUpgrade(ctx, w, args)
	case "daemon-install":
		duplicateLogWriter(ctx, w)

		return daemonInstall(ctx, w, args)
	case "daemon-upgrade":
		duplicateLogWriter(ctx, w)

		return daemonUpgrade(ctx, w, args)
	case "service-command":
		duplicateLogWriter(ctx, w)

		return serviceCommand(ctx, w, args)
	}

	return errors.New("unknown command")
}

func duplicateLogWriter(ctx context.Context, w io.Writer) {
	oldLogWriter := log.Writer()
	mw := io.MultiWriter(w, oldLogWriter)
	log.SetOutput(mw)

	go func() {
		<-ctx.Done()
		log.SetOutput(oldLogWriter)
	}()
}

//nolint:unparam
func nodeInfo(ctx context.Context, w io.Writer, _ []string) error {
	info := contextInternal.OSInfoFromContext(ctx)
	_, _ = w.Write([]byte(info.String()))

	return nil
}

var replaceMap = map[string]string{
	"daemon":       gameapDaemonService,
	"gameapdaemon": gameapDaemonService,
	"gameap":       gameapServiceName,
	"mysql":        "mariadb",
	"php":          "php-fpm",
}

func serviceStatus(ctx context.Context, w io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("no service name provided")
	}

	serviceName := strings.ToLower(args[0])

	if replace, ok := replaceMap[serviceName]; ok {
		serviceName = replace
	}

	if serviceName == gameapServiceName {
		return gameapStatus(ctx, w, args)
	}

	if serviceName == gameapDaemonService {
		return daemonStatus(ctx, w, args)
	}

	var errNotFound *service.NotFoundError
	err := service.Status(ctx, serviceName)
	if err != nil && errors.Is(err, service.ErrInactiveService) {
		_, _ = w.Write([]byte(serviceStatusInactive))

		return nil
	}
	if err != nil && !errors.As(err, &errNotFound) {
		return errors.WithMessage(err, "failed to get service status")
	}
	if errNotFound != nil {
		_, _ = w.Write([]byte(serviceStatusNotFound))

		//nolint:nilerr
		return nil
	}

	_, _ = w.Write([]byte(serviceStatusActive))

	return nil
}

func gameapStatus(ctx context.Context, w io.Writer, _ []string) error {
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to get panel install state"))

		_, _ = w.Write([]byte(serviceStatusNotFound))

		return nil
	}

	if state.Version == "" || state.Version == "3" || state.Version == "v3" {
		log.Println("Checking GameAP v3 status")

		return gameapStatusV3(ctx, w, state)
	}

	if state.Version == "v4" || state.Version == "4" {
		log.Println("Checking GameAP v4 status")

		return gameapStatusV4(ctx, w, state)
	}

	return nil
}

func gameapStatusV3(_ context.Context, w io.Writer, state gameapctl.PanelInstallState) error {
	if state.Path == "" {
		log.Println("No installation path found in state, using default path")

		state.Path = gameap.DefaultWebInstallationPath
	}

	if utils.IsFileExists(state.Path) {
		log.Println("GameAP v3 installation found at path:", state.Path)

		_, _ = w.Write([]byte(serviceStatusActive))

		return nil
	}

	log.Println("GameAP v3 installation not found at path:", state.Path)

	_, _ = w.Write([]byte(serviceStatusNotFound))

	return nil
}

func gameapStatusV4(ctx context.Context, w io.Writer, state gameapctl.PanelInstallState) error {
	if state.DataDirectory == "" {
		log.Println("No data directory found in state, using default path")

		state.DataDirectory = gameap.DefaultDataPath
	}

	if state.ConfigDirectory == "" {
		log.Println("No config directory found in state, using default path")

		state.ConfigDirectory = gameap.DefaultConfigFilePath
	}

	if !utils.IsFileExists(state.DataDirectory) {
		log.Println("Data directory not found:", state.DataDirectory)

		_, _ = w.Write([]byte(serviceStatusNotFound))

		return nil
	}

	if !utils.IsFileExists(state.ConfigDirectory) {
		log.Println("Config directory not found:", state.ConfigDirectory)

		_, _ = w.Write([]byte(serviceStatusNotFound))

		return nil
	}

	pr, err := oscore.FindProcessByName(ctx, gameapProcessName)
	if err != nil {
		_, _ = w.Write([]byte(serviceStatusNotFound))

		log.Println(errors.WithMessage(err, "failed to find started gameap process"))

		return nil
	}

	if pr == nil {
		_, _ = w.Write([]byte(serviceStatusInactive))

		return nil
	}

	_, _ = w.Write([]byte(serviceStatusActive))

	return nil
}

func daemonStatus(ctx context.Context, w io.Writer, _ []string) error {
	path := "gameap-daemon"
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	_, err := exec.LookPath(path)
	if err != nil {
		log.Println(errors.WithMessage(err, "gameap-daemon not found"))
		_, _ = w.Write([]byte(serviceStatusNotFound))

		return nil
	}

	err = daemon.Status(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "daemon status failed"))
		_, _ = w.Write([]byte(serviceStatusInactive))

		return nil
	}

	_, _ = w.Write([]byte(serviceStatusActive))

	return nil
}

func gameapInstall(ctx context.Context, w io.Writer, args []string) error {
	// Testing
	if len(args) == 0 {
		return errors.New("no args")
	}

	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	exArgs := append([]string{"--non-interactive", "panel", "install"}, args...)

	cmd := exec.Command(ex, exArgs...)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	packagemanager.UpdateEnvPath(ctx)

	return nil
}

func gameapUninstall(ctx context.Context, w io.Writer, args []string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	exArgs := append([]string{"--non-interactive", "panel", "uninstall"}, args...)

	cmd := exec.Command(ex, exArgs...)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	packagemanager.UpdateEnvPath(ctx)

	return nil
}

func serviceCommand(ctx context.Context, w io.Writer, args []string) error {
	if len(args) < 2 {
		return errors.New("not enough arguments, should be at least 2: command and service name")
	}

	command := args[0]
	serviceName := strings.ToLower(args[1])

	if replace, ok := replaceMap[serviceName]; ok {
		serviceName = replace
	}

	if serviceName == gameapServiceName {
		return errors.New("gameap service command is not supported")
	}

	if serviceName == gameapDaemonService {
		return daemonCommand(ctx, w, []string{command})
	}

	log.Printf("Service command: %s %s", command, serviceName)

	var err error
	switch command {
	case "start":
		err = service.Start(ctx, serviceName)
	case "stop":
		err = service.Stop(ctx, serviceName)
	case "restart":
		err = service.Restart(ctx, serviceName)
	default:
		return errors.New("unknown command")
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start service")
	}

	return nil
}

func daemonCommand(ctx context.Context, _ io.Writer, args []string) error {
	if len(args) < 1 {
		return errors.New("no command provided")
	}

	command := args[0]

	var err error
	switch command {
	case "start":
		err = daemon.Start(ctx)
	case "stop":
		err = daemon.Stop(ctx)
	case "restart":
		err = daemon.Restart(ctx)
	default:
		return errors.New("unknown command")
	}

	if err != nil {
		return errors.WithMessage(err, "failed to execute command")
	}

	return nil
}

func gameapUpgrade(_ context.Context, w io.Writer, _ []string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	cmd := exec.Command(ex, "--non-interactive", "panel", "upgrade")
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	return nil
}

//nolint:funlen
func daemonInstall(ctx context.Context, w io.Writer, args []string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	host := ""
	installationToken := ""

	if len(args) < 2 {
		return errors.New("not enough arguments, should be at least 2: host and installation token")
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "--host=") {
			host = strings.TrimPrefix(arg, "--host=")
		}

		if strings.HasPrefix(arg, "--installation-token=") {
			installationToken = strings.TrimPrefix(arg, "--installation-token=")
		}
	}

	if host == "" || installationToken == "" {
		return errors.New("host and installation token should be provided")
	}

	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}

	client := http.DefaultClient
	client.Timeout = 5 * time.Second //nolint:mnd

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/gdaemon/setup/%s", host, installationToken),
		nil,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	//nolint:bodyclose
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to get daemon setup")
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to get daemon setup")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	createToken := ""

	split := strings.Split(string(body), ";")
	if len(split) < 2 {
		return errors.New("invalid response body")
	}

	for _, s := range split {
		switch {
		case strings.HasPrefix(s, "export createToken="):
			createToken = strings.TrimPrefix(s, "export createToken=")
		case strings.HasPrefix(s, "export CREATE_TOKEN="):
			createToken = strings.TrimPrefix(s, "export CREATE_TOKEN=")
		}
	}

	if createToken == "" {
		return errors.New("failed to get create token")
	}

	exPath := filepath.Dir(ex)

	cmd := exec.Command(
		ex,
		"--non-interactive",
		"daemon",
		"install",
		"--host="+host,
		"--token="+createToken,
	)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	packagemanager.UpdateEnvPath(ctx)

	return nil
}

func daemonUpgrade(_ context.Context, w io.Writer, _ []string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	cmd := exec.Command(ex, "--non-interactive", "daemon", "upgrade")
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	return nil
}
