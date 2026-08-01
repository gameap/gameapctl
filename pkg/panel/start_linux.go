//go:build linux

package panel

import (
	"bufio"
	"context"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/systemd"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const panelServiceName = "gameap"

func Start(ctx context.Context, opts ...Options) error {
	o := firstOptions(opts)

	// DetectInit reads /proc/1/exe, which an unprivileged user cannot do, so in user
	// scope the init system must not be probed at all.
	if o.scope() == gameap.ScopeUser {
		return startPanelSystemdScope(ctx, gameap.ScopeUser)
	}

	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = startPanelSystemdScope(ctx, gameap.ScopeSystem)
	case runhelper.InitUnknown:
		err = startFork(ctx, gameap.ScopeSystem)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start GameAP")
	}

	return nil
}

func startPanelSystemdScope(ctx context.Context, scope string) error {
	paths, err := gameap.PanelPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve panel paths")
	}

	_, statErr := os.Stat(paths.SystemdUnitPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		return ErrGameAPNotInstalled
	}
	if statErr != nil {
		return errors.WithMessage(statErr, "failed to stat gameap service configuration")
	}

	err = systemd.Start(ctx, paths.Scope, panelServiceName)
	if err != nil {
		return errors.WithMessage(err, "failed to start gameap")
	}

	return nil
}

func readEnvFromFile(configPath string) ([]string, error) {
	envVars := os.Environ()

	file, err := os.Open(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Println("Config file not found, using system environment only")

			return envVars, nil
		}

		return nil, errors.WithMessage(err, "failed to open config file")
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println("Failed to close config file:", err)
		}
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		if strings.Contains(line, "=") {
			envVars = append(envVars, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WithMessage(err, "failed to read config file")
	}

	return envVars, nil
}

func startFork(ctx context.Context, scope string) error {
	log.Println("Starting GameAP (fork)")

	paths, err := gameap.PanelPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve panel paths")
	}

	proc, err := oscore.FindProcessByName(ctx, processName)
	if err != nil {
		return errors.WithMessage(err, "failed to find gameap")
	}

	if proc != nil && proc.Pid != 0 {
		return errors.New("gameap is already running")
	}

	exePath, err := lookupPanelBinary(paths)
	if err != nil {
		return err
	}
	log.Println("Found", exePath)

	if _, err := os.Stat(paths.DataDir); errors.Is(err, fs.ErrNotExist) {
		err := os.MkdirAll(paths.DataDir, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create work path")
		}
	}

	// Open /dev/null for stdin, stdout, stderr
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return errors.WithMessage(err, "failed to open /dev/null")
	}
	defer func(devNull *os.File) {
		err := devNull.Close()
		if err != nil {
			log.Println("Failed to close /dev/null:", err)
		}
	}(devNull)

	envVars, err := readEnvFromFile(paths.ConfigFilePath)
	if err != nil {
		return errors.WithMessage(err, "failed to read environment variables")
	}

	attr := os.ProcAttr{
		Dir: paths.DataDir,
		Env: envVars,
		Sys: &syscall.SysProcAttr{
			Setsid: true, // Create a new session and detach from terminal
		},
		Files: []*os.File{devNull, devNull, devNull},
	}
	p, err := os.StartProcess(exePath, []string{exePath}, &attr)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to start process"))

		return errors.WithMessage(err, "failed to start process")
	}

	log.Println("Process started with pid", p.Pid)

	// Start a goroutine to wait for the process and reap it when it terminates
	// This prevents zombie processes from accumulating
	go func() {
		state, waitErr := p.Wait()
		if waitErr != nil {
			log.Printf("Error waiting for process (pid %d): %v\n", p.Pid, waitErr)

			return
		}
		log.Printf("Process (pid %d) exited with status: %s\n", p.Pid, state.String())
	}()

	return nil
}

// lookupPanelBinary prefers the scope specific path over PATH: ~/.local/bin is
// frequently missing from PATH in non-login shells.
func lookupPanelBinary(paths gameap.PanelPaths) (string, error) {
	if utils.IsFileExists(paths.BinaryPath) {
		return paths.BinaryPath, nil
	}

	exePath, err := exec.LookPath(processName)
	if err != nil {
		return "", errors.WithMessage(err, "failed to lookup gameap path")
	}

	return exePath, nil
}
