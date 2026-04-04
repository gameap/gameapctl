package sendlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	tmpDir, err := os.MkdirTemp("", "gameapctl-send-logs")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp file")
	}
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove temporary directory"))
		}
	}()

	collectAllLogs(ctx, tmpDir, cliCtx.StringSlice("include-logs"))

	fmt.Println("Compressing logs...")
	f, err := os.CreateTemp("", "gameapctl-send-logs")
	if err != nil {
		return err
	}
	defer func() {
		err := os.Remove(f.Name())
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to remove temporary archive file"))
		}
	}()

	err = compress(tmpDir, f)
	if err != nil {
		return errors.WithMessage(err, "failed to compress logs")
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return errors.WithMessage(err, "failed to seek file")
	}

	fmt.Println("Sending logs...")
	id, err := sendFile(ctx, f)
	if err != nil {
		return errors.WithMessage(err, "failed to send file")
	}
	err = f.Close()
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to close temporary archive file"))
	}

	fmt.Println()
	fmt.Println("--------------------------")
	fmt.Println("Logs was sent")
	fmt.Println("Logs ID:", id)
	fmt.Println("Please, send this ID to GameAP support")
	fmt.Println("Telegram: https://t.me/gameap")

	return nil
}

func collectAllLogs(ctx context.Context, tmpDir string, additionalLogs []string) {
	collectors := []struct {
		name string
		fn   func() error
	}{
		{"gameapctl logs", func() error { return collectGameapCTLLogs(ctx, tmpDir) }},
		{"daemon logs", func() error { return collectDaemonLogs(ctx, tmpDir) }},
		{"journal logs", func() error { return collectJournalLogs(ctx, tmpDir) }},
		{"panel logs", func() error { return collectPanelLogs(ctx, tmpDir) }},
		{"web server logs", func() error { return collectWebServerLogs(ctx, tmpDir) }},
		{"database logs", func() error { return collectDatabaseLogs(ctx, tmpDir) }},
		{"system information", func() error { return collectSystemInfo(ctx, tmpDir) }},
	}

	for _, c := range collectors {
		fmt.Printf("Collecting %s...\n", c.name)
		if err := c.fn(); err != nil {
			log.Println(errors.WithMessagef(err, "failed to collect %s", c.name))
		}
	}

	if len(additionalLogs) > 0 {
		fmt.Println("Collecting additional logs...")
		if err := collectAdditionalLogs(ctx, additionalLogs, tmpDir); err != nil {
			log.Println(errors.WithMessage(err, "failed to collect additional logs"))
		}
	}
}

func collectGameapCTLLogs(_ context.Context, destinationDir string) error {
	if !utils.IsFileExists(logsPathGamectl) {
		// skip
		return nil
	}

	destinationDir = filepath.Join(destinationDir, "gameapctl")
	err := os.Mkdir(destinationDir, 0755)
	if err != nil {
		return errors.WithMessage(err, "failed to create daemon logs directory")
	}

	err = utils.Copy(logsPathGamectl, destinationDir)
	if err != nil {
		return errors.WithMessage(err, "failed to copy gameapctl logs")
	}

	err = utils.ChownR(destinationDir, 1000, 1000) //nolint:mnd
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change owner"))
	}

	err = os.Chmod(destinationDir, 0755)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change permissions"))
	}

	return nil
}

func collectDaemonLogs(_ context.Context, destinationDir string) error {
	if !utils.IsFileExists(logsPathDaemon) {
		// skip
		return nil
	}

	destinationDir = filepath.Join(destinationDir, "gameap-daemon")
	err := os.Mkdir(destinationDir, 0755)
	if err != nil {
		return errors.WithMessage(err, "failed to create daemon logs directory")
	}

	err = utils.Copy(logsPathDaemon, destinationDir)
	if err != nil {
		return errors.WithMessage(err, "failed to copy daemon logs")
	}

	err = utils.ChownR(destinationDir, 1000, 1000) //nolint:mnd
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change owner"))
	}

	err = os.Chmod(destinationDir, 0755)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change permissions"))
	}

	return nil
}

func collectPanelLogs(ctx context.Context, destinationDir string) error {
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		// Just log the error and continue
		log.Println(errors.WithMessage(err, "failed to load panel install state"))
	}

	destinationDir = filepath.Join(destinationDir, "panel")

	panelPath := state.Path
	if panelPath == "" {
		panelPath = defaultPanelInstallPath
	}

	if !utils.IsFileExists(panelPath) {
		// skip
		return nil
	}

	logPath := filepath.Join(panelPath, "storage", "logs")
	if !utils.IsFileExists(logPath) {
		// skip
		return nil
	}

	err = utils.Copy(logPath, destinationDir)
	if err != nil {
		return errors.WithMessage(err, "failed to copy gameap logs")
	}

	err = utils.ChownR(destinationDir, 1000, 1000) //nolint:mnd
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change owner"))
	}

	err = os.Chmod(destinationDir, 0755)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change permissions"))
	}

	return nil
}

func collectSystemInfo(ctx context.Context, destinationDir string) error {
	f, err := os.OpenFile(filepath.Join(destinationDir, "system_info.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.WithMessage(err, "failed to open file")
	}
	defer func() {
		err := f.Close()
		if err != nil {
			return
		}
	}()

	osInfo := contextInternal.OSInfoFromContext(ctx)

	_, _ = f.WriteString("GameAPCtl Version: " + gameap.Version + "\n")
	_, _ = f.WriteString("GameAPCtl Build Date: " + gameap.BuildDate + "\n")
	_, _ = f.WriteString("Kernel: " + osInfo.Kernel + "\n")
	_, _ = f.WriteString("Core: " + osInfo.Core + "\n")
	_, _ = f.WriteString("Distribution: " + string(osInfo.Distribution) + "\n")
	_, _ = f.WriteString("DistributionVersion: " + osInfo.DistributionVersion + "\n")
	_, _ = f.WriteString("DistributionCodename: " + osInfo.DistributionCodename + "\n")
	_, _ = f.WriteString("Platform: " + osInfo.Platform.String() + "\n")
	_, _ = f.WriteString("OS: " + osInfo.OS + "\n")
	_, _ = f.WriteString("Hostname: " + osInfo.Hostname + "\n")

	state, err := gameapctl.LoadPanelInstallState(ctx)
	//nolint:nestif
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load panel install state"))
	} else {
		if state.Host != "" {
			_, _ = f.WriteString("Host: " + state.Host + "\n")
		}
		if state.HostIP != "" {
			_, _ = f.WriteString("Host IP: " + state.HostIP + "\n")
		}
		if state.Port != "" {
			_, _ = f.WriteString("Port: " + state.Port + "\n")
		}
		if state.Path != "" {
			_, _ = f.WriteString("Path: " + state.Path + "\n")
		}
		if state.WebServer != "" {
			_, _ = f.WriteString("Web server: " + state.WebServer + "\n")
		}
		if state.Database != "" {
			_, _ = f.WriteString("Database: " + state.Database + "\n")
			if state.DatabaseWasInstalled {
				_, _ = f.WriteString("Database was installed\n")
			}
		}
	}

	return nil
}

//nolint:unparam
func collectAdditionalLogs(_ context.Context, logs []string, destinationDir string) error {
	destinationDir = filepath.Join(destinationDir, "additional")

	for _, logPath := range logs {
		if !utils.IsFileExists(logPath) {
			// skip
			continue
		}

		dest := filepath.Join(destinationDir, filepath.Base(logPath))

		err := utils.Copy(logPath, dest)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to copy %s", logPath))
		}
	}

	err := utils.ChownR(destinationDir, 1000, 1000) //nolint:mnd
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change owner"))
	}

	err = os.Chmod(destinationDir, 0755)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to change permissions"))
	}

	return nil
}

func sendFile(ctx context.Context, buf io.Reader) (string, error) {
	req, err := http.NewRequest(http.MethodPost, apiPath, buf)
	if err != nil {
		return "", errors.WithMessage(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/tar+gzip")
	req = req.WithContext(ctx)

	client := &http.Client{
		Timeout: 5 * time.Minute, //nolint:mnd
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.WithMessage(err, "failed to send logs")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return "", errors.Errorf("failed to send logs: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", errors.WithMessage(err, "failed to decode response")
	}

	return result.ID, nil
}
