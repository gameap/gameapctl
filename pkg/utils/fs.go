package utils

import (
	"bufio"
	"context"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/otiai10/copy"
	"github.com/pkg/errors"
)

func Move(src string, dst string) error {
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return errors.Errorf("source file %s not found", src)
	}
	dstDir := filepath.Dir(dst)
	if _, err := os.Stat(dstDir); errors.Is(err, fs.ErrNotExist) {
		err = os.MkdirAll(dstDir, 0755)
		if err != nil {
			return errors.WithMessagef(err, "failed to create destination directory %s", dst)
		}
	}
	return os.Rename(src, dst)
}

func Copy(src string, dst string) error {
	return copy.Copy(src, dst)
}

func WriteContentsToFile(contents []byte, path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println(err)
		}
	}(file)

	_, err = file.Write(contents)
	if err != nil {
		return err
	}

	return nil
}

//nolint:funlen,gocognit
func FindLineAndReplace(ctx context.Context, path string, replaceMap map[string]string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil && !errors.Is(err, fs.ErrClosed) {
			log.Println(err)
		}
	}(file)

	tmpFile, err := os.CreateTemp("", "find-and-replace")
	if err != nil {
		return err
	}
	defer func(tmpFile *os.File) {
		err := tmpFile.Close()
		if err != nil && !errors.Is(err, fs.ErrClosed) {
			log.Println(err)
		}
	}(tmpFile)

	reader := bufio.NewReader(file)

	err = findLineAndReplace(ctx, reader, tmpFile, replaceMap)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}
	err = tmpFile.Close()
	if err != nil {
		return err
	}
	err = os.Rename(tmpFile.Name(), path)
	if err != nil {
		return err
	}

	return nil
}

func findLineAndReplace(_ context.Context, r io.Reader, w io.Writer, replaceMap map[string]string) error {
	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(w)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		for needle, replacement := range replaceMap {
			needleLen := len(needle)
			if len(line) <= needleLen {
				continue
			}
			if line[:needleLen] == needle {
				line = replacement + "\n"
				continue
			}
		}

		_, err = writer.WriteString(line)
		if err != nil {
			return err
		}
	}

	return writer.Flush()
}
