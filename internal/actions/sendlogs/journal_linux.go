//go:build linux

package sendlogs

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

var journalUnits = []string{"gameap", "gameap-daemon"}

func collectJournalLogs(ctx context.Context, destinationDir string) error {
	initSystem, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to detect init system"))

		return nil
	}
	if initSystem != runhelper.InitSystemd {
		return nil
	}

	if _, err := exec.LookPath("journalctl"); err != nil {
		return err
	}

	destinationDir = filepath.Join(destinationDir, "journal")
	err = os.Mkdir(destinationDir, 0755)
	if err != nil {
		return errors.WithMessage(err, "failed to create journal logs directory")
	}

	for _, unit := range journalUnits {
		output, err := oscore.ExecCommandWithOutput(ctx, "journalctl", "-u", unit, "--no-pager", "-n", "1000")
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to get journal for %s", unit))

			continue
		}

		filePath := filepath.Join(destinationDir, unit+".log")
		err = os.WriteFile(filePath, []byte(output), 0600)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to write journal log for %s", unit))

			continue
		}
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
