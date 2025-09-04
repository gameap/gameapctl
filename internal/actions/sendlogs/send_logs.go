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

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

//nolint:funlen
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

	err = collectGameapCTLLogs(ctx, tmpDir)
	if err != nil {
		return errors.WithMessage(err, "failed to collect gameapctl logs")
	}

	err = collectDaemonLogs(ctx, tmpDir)
	if err != nil {
		return errors.WithMessage(err, "failed to collect daemon logs")
	}

	err = collectPanelLogs(ctx, tmpDir)
	if err != nil {
		return errors.WithMessage(err, "failed to collect panel logs")
	}

	err = collectSystemInfo(ctx, tmpDir)
	if err != nil {
		return errors.WithMessage(err, "failed to collect system info")
	}

	additionalLogs := cliCtx.StringSlice("include-logs")
	if len(additionalLogs) > 0 {
		err = collectAdditionalLogs(ctx, additionalLogs, tmpDir)
		if err != nil {
			return errors.WithMessage(err, "failed to collect additional logs")
		}
	}

	f, err := os.CreateTemp("", "gameapctl-send-logs")
	if err != nil {
		return err
	}
	err = compress(tmpDir, f)
	if err != nil {
		return errors.WithMessage(err, "failed to compress logs")
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return errors.WithMessage(err, "failed to seek file")
	}

	var id string
	id, err = sendFile(ctx, f)
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

	_, err = f.WriteString("Kernel: " + osInfo.Kernel + "\n")
	if err != nil {
		return errors.WithMessage(err, "failed to write to file")
	}
	_, _ = f.WriteString("Core: " + osInfo.Core + "\n")
	_, _ = f.WriteString(string("Distribution: " + osInfo.Distribution + "\n"))
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.WithMessage(err, "failed to send logs")
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("failed to send logs")
	}

	var result struct {
		ID string `json:"id"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", errors.WithMessage(err, "failed to decode response")
	}
	err = resp.Body.Close()
	if err != nil {
		log.Println("failed to close body", err)
	}

	return result.ID, nil
}
