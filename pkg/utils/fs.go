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
		return moveCrossDevice(src, dst)
	}

	err = os.Rename(src, dst)
	if err != nil && strings.Contains(err.Error(), "cross-device link") {
		return moveCrossDevice(src, dst)
	}

	return err
}

func moveCrossDevice(src string, dst string) error {
	err := copy.Copy(src, dst)
	if err != nil {
		return errors.WithMessage(err, "failed to copy files")
	}

	err = os.RemoveAll(src)
	if err != nil {
		return errors.WithMessage(err, "failed to remove files from source directory")
	}

	return nil
}

func Copy(src string, dst string) error {
	return copy.Copy(src, dst)
}

// ReplaceFile moves src over dst without truncating dst in place: the file is staged
// under a unique name next to dst and renamed into it, so a process still executing dst
// keeps its old inode and no reader ever sees a half-written file.
func ReplaceFile(src, dst string, mode os.FileMode) error {
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create destination directory %s", dstDir)
	}

	stagingFile, err := os.CreateTemp(dstDir, filepath.Base(dst)+".*.new")
	if err != nil {
		return errors.Wrapf(err, "failed to create staging file for %s", dst)
	}

	staging := stagingFile.Name()
	if err := stagingFile.Close(); err != nil {
		_ = os.Remove(staging)

		return errors.Wrapf(err, "failed to close staging file %s", staging)
	}

	if err := Move(src, staging); err != nil {
		_ = os.Remove(staging)

		return errors.WithMessagef(err, "failed to stage %s", dst)
	}

	if err := os.Chmod(staging, mode); err != nil {
		_ = os.Remove(staging)

		return errors.Wrapf(err, "failed to set permissions on %s", staging)
	}

	if err := os.Rename(staging, dst); err != nil {
		_ = os.Remove(staging)

		return errors.Wrapf(err, "failed to replace %s", dst)
	}

	return nil
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

func AppendContentsToFile(contents []byte, path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
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

	uid, gid := FileOwner(path)

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
	err = Move(tmpFile.Name(), path)
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

//nolint:gocognit
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

// TailFile returns the last maxLines lines of the file, reading no more than maxBytes from its end.
// Non-positive maxLines or maxBytes disables the corresponding limit.
func TailFile(path string, maxLines int, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open file %s", path)
	}
	defer func() {
		_ = f.Close()
	}()

	stat, err := f.Stat()
	if err != nil {
		return "", errors.Wrapf(err, "failed to stat file %s", path)
	}

	var offset int64
	if maxBytes > 0 && stat.Size() > maxBytes {
		offset = stat.Size() - maxBytes
	}

	// The byte before the offset shows whether the read starts in the middle of a line;
	// such an incomplete first line is dropped after reading.
	firstLineIncomplete := false
	if offset > 0 {
		prev := make([]byte, 1)
		if _, readErr := f.ReadAt(prev, offset-1); readErr != nil || prev[0] != '\n' {
			firstLineIncomplete = true
		}
	}

	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return "", errors.Wrapf(err, "failed to seek file %s", path)
	}

	var reader io.Reader = f
	if maxBytes > 0 {
		reader = io.LimitReader(f, maxBytes)
	}

	contents, err := io.ReadAll(reader)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read file %s", path)
	}

	text := strings.ReplaceAll(string(contents), "\r\n", "\n")
	// A newline right at the offset means the incomplete part is empty.
	if strings.HasPrefix(text, "\n") {
		firstLineIncomplete = false
	}
	text = strings.Trim(text, "\n")
	if text == "" {
		return "", nil
	}

	lines := strings.Split(text, "\n")

	if firstLineIncomplete && len(lines) > 1 {
		lines = lines[1:]
	}

	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return strings.Join(lines, "\n"), nil
}
