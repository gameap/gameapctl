package sendlogs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	// Shawl keeps up to 60 daily log files per service, so the logs are limited
	// both by age and by size to keep the archive sendable.
	serviceLogMaxAge  = 14 * 24 * time.Hour
	serviceLogMaxSize = 2 * 1024 * 1024
)

// copyLogDir copies log files, skipping the outdated ones and keeping only the tail
// of the files that are too large to be sent.
func copyLogDir(src string, destinationDir string, deadline time.Time) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return errors.Wrapf(err, "failed to read directory %s", src)
	}

	err = os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "failed to create directory %s", destinationDir)
	}

	for _, entry := range entries {
		path := filepath.Join(src, entry.Name())
		destination := filepath.Join(destinationDir, entry.Name())

		if entry.IsDir() {
			if err = copyLogDir(path, destination, deadline); err != nil {
				log.Println(errors.WithMessagef(err, "failed to copy directory %s", path))
			}

			continue
		}

		info, err := entry.Info()
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to get info of %s", path))

			continue
		}

		if info.ModTime().Before(deadline) {
			continue
		}

		if err = copyLogFile(path, destination, info.Size()); err != nil {
			log.Println(errors.WithMessagef(err, "failed to copy %s", path))
		}
	}

	return nil
}

func copyLogFile(path string, destination string, size int64) error {
	if size <= serviceLogMaxSize {
		return utils.Copy(path, destination)
	}

	tail, err := utils.TailFile(path, 0, serviceLogMaxSize)
	if err != nil {
		return err
	}

	contents := fmt.Sprintf("... truncated, only the last %d bytes are kept ...\n%s\n", serviceLogMaxSize, tail)

	err = os.WriteFile(destination, []byte(contents), 0600)
	if err != nil {
		return errors.Wrapf(err, "failed to write %s", destination)
	}

	return nil
}

func filterPortLines(output string, port string) string {
	needle := ":" + port
	lines := strings.Split(output, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		index := strings.Index(line, needle)
		if index < 0 {
			continue
		}

		rest := line[index+len(needle):]
		if rest != "" && !unicode.IsSpace(rune(rest[0])) {
			continue
		}

		result = append(result, strings.TrimRight(line, "\r"))
	}

	return strings.Join(result, "\n")
}
