package utils

import (
	"bufio"
	"context"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/otiai10/copy"
	"github.com/pkg/errors"
)

func IsFileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func Move(src string, dst string) error {
	_, err := os.Stat(src)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return errors.WithMessagef(err, "source file %s not found", src)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to stat src file")
	}

	dstDir := filepath.Dir(dst)
	_, err = os.Stat(dstDir)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		log.Printf("creating '%s' directory\n", dstDir)
		err = os.MkdirAll(dstDir, 0755)
		if err != nil {
			return errors.WithMessagef(err, "failed to create destination directory %s", dst)
		}
	}
	if err != nil {
		return errors.WithMessage(err, "failed to stat destination directory")
	}

	if runtime.GOOS == "windows" {
		err = copy.Copy(src, dst)
		if err != nil {
			return errors.WithMessage(err, "failed to copy files")
		}

		err = os.RemoveAll(src)
		if err != nil {
			return errors.WithMessage(err, "failed to remove files from source directory")
		}

		return nil
	}

	return os.Rename(src, dst)
}

func Copy(src string, dst string) error {
	return copy.Copy(src, dst)
}

func WriteContentsToFile(contents []byte, path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
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

func FindLineAndReplace(ctx context.Context, path string, replaceMap map[string]string) error {
	return findInFileAndReplaceOrAdd(ctx, path, replaceMap, false)
}

func FindLineAndReplaceOrAdd(ctx context.Context, path string, replaceMap map[string]string) error {
	return findInFileAndReplaceOrAdd(ctx, path, replaceMap, true)
}

func findInFileAndReplaceOrAdd(ctx context.Context, path string, replaceMap map[string]string, add bool) error {
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

	uid, gid := uidAndGIDForFile(path)

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

	err = findLineAndReplaceOrAdd(ctx, reader, tmpFile, replaceMap, add)
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

	if uid != 0 && gid != 0 {
		err = os.Chown(path, int(uid), int(gid))
		if err != nil {
			return err
		}
	}

	return nil
}

//nolint:funlen,gocognit
func findLineAndReplaceOrAdd(
	_ context.Context,
	r io.Reader,
	w io.Writer,
	replaceMap map[string]string,
	add bool,
) error {
	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(w)

	for {
		b, isPrefix, err := reader.ReadLine()
		line := string(b)
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if isPrefix {
			return errors.New("buffer size is too small")
		}

		for needle, replacement := range replaceMap {
			needleLen := len(needle)
			trimmedLine := strings.TrimSpace(line)

			equal := false
			matched := false

			if len(trimmedLine) >= needleLen {
				equal = trimmedLine[:needleLen] == needle
			}

			if !equal {
				matched, err = regexp.MatchString(needle, trimmedLine)
				if err != nil {
					return err
				}
			}

			if equal || matched {
				fi := strings.Index(line, trimmedLine)
				li := strings.LastIndex(line, trimmedLine)

				b := strings.Builder{}
				b.Grow(len(line) + len(replacement))
				b.WriteString(line[:fi])
				b.WriteString(replacement)
				b.WriteString(line[li+len(trimmedLine):])

				line = b.String()

				delete(replaceMap, needle)

				break
			}
		}

		_, err = writer.WriteString(line)
		if err != nil {
			return err
		}
		err = writer.WriteByte('\n')
		if err != nil {
			return err
		}
	}

	if add {
		for _, replacement := range replaceMap {
			_, err := writer.WriteString(replacement)
			if err != nil {
				return err
			}
			err = writer.WriteByte('\n')
			if err != nil {
				return err
			}
		}
	}

	return writer.Flush()
}
