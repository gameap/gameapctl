//go:build linux

package panel

import (
	"bufio"
	"context"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

const (
	systemdConfigPath = "/etc/systemd/system/gameap.service"
)

func Start(ctx context.Context) error {
	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = startSystemd(ctx)
	case runhelper.InitUnknown:
		err = startFork(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start GameAP")
	}

	return nil
}

func startSystemd(ctx context.Context) error {
	_, err := os.Stat(systemdConfigPath)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return ErrGameAPNotInstalled
	}
	if err != nil {
		return errors.WithMessage(err, "failed to stat gameap service configuration")
	}

	err = service.Start(ctx, "gameap")
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

func startFork(ctx context.Context) error {
	log.Println("Starting GameAP (fork)")

	proc, err := oscore.FindProcessByName(ctx, processName)
	if err != nil {
		return errors.WithMessage(err, "failed to find gameap")
	}

	if proc != nil && proc.Pid != 0 {
		return errors.New("gameap is already running")
	}

	exePath, err := exec.LookPath("gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to lookup gameap path")
	}
	log.Println("Found", exePath)

	if _, err := os.Stat(defaultDataDir); errors.Is(err, fs.ErrNotExist) {
		err := os.Mkdir(defaultDataDir, 0755)
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

	envVars, err := readEnvFromFile(filepath.Join(defaultConfigDir, "config.env"))
	if err != nil {
		return errors.WithMessage(err, "failed to read environment variables")
	}

	attr := os.ProcAttr{
		Dir: defaultDataDir,
		Env: envVars,
		Sys: &syscall.SysProcAttr{
			Setsid: true, // Create a new session and detach from terminal
		},
		Files: []*os.File{devNull, devNull, devNull},
	}
	p, err := os.StartProcess(exePath, []string{}, &attr)
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

type SystemdUnitConfig struct {
	User             string
	Group            string
	WorkingDirectory string
	ExecStart        string
	EnvironmentFile  string
	ReadWritePaths   string
}
