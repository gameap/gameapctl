//go:build windows

package oscore

import (
	"bytes"
	"io"
	"math"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

const (
	utf8CodePage = 65001

	// maxBufferedOutput limits how much output is kept while waiting for a line break.
	maxBufferedOutput = 64 * 1024
)

var errEmptyDecodeResult = errors.New("empty decode result")

// OutputDecoder converts the output of a console application to UTF-8.
//
// Console applications write their output in the OEM code page of the console (CP850, CP852,
// CP866 and so on), while gameapctl writes its log in UTF-8. Without the conversion every
// non-ASCII character of a command output becomes unreadable in the log.
//
// Output is buffered until a line break because a single character may be split between
// two writes. Flush must be called when the command is finished.
type OutputDecoder struct {
	writer   io.Writer
	codePage uint32
	buf      bytes.Buffer
}

// NewOutputDecoder wraps writer with a decoder of the console output code page.
func NewOutputDecoder(writer io.Writer) *OutputDecoder {
	codePage, err := windows.GetConsoleOutputCP()
	if err != nil {
		codePage = 0
	}

	return &OutputDecoder{writer: writer, codePage: codePage}
}

// Write decodes and writes the complete lines of p, the rest is buffered until the next write.
func (d *OutputDecoder) Write(p []byte) (int, error) {
	if d.codePage == 0 || d.codePage == utf8CodePage {
		n, err := d.writer.Write(p)

		return n, errors.Wrap(err, "failed to write command output")
	}

	d.buf.Write(p)

	for {
		lineLen := bytes.IndexByte(d.buf.Bytes(), '\n') + 1
		if lineLen == 0 {
			if d.buf.Len() < maxBufferedOutput {
				break
			}

			lineLen = d.buf.Len()
		}

		err := d.writeDecoded(d.buf.Next(lineLen))
		if err != nil {
			return len(p), err
		}
	}

	return len(p), nil
}

// Flush writes out the output that is not terminated with a line break yet.
func (d *OutputDecoder) Flush() {
	if d.buf.Len() == 0 {
		return
	}

	_ = d.writeDecoded(d.buf.Next(d.buf.Len()))
}

func (d *OutputDecoder) writeDecoded(line []byte) error {
	decoded := line

	// Valid UTF-8 output is left as is: tools like curl and tar already write UTF-8, and
	// bytes of the OEM code pages almost never form a valid UTF-8 sequence.
	if !utf8.Valid(line) {
		converted, err := decodeCodePage(d.codePage, line)
		if err == nil {
			decoded = converted
		}
	}

	_, err := d.writer.Write(decoded)

	return errors.Wrap(err, "failed to write command output")
}

func decodeCodePage(codePage uint32, data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > math.MaxInt32 {
		return nil, errEmptyDecodeResult
	}

	//nolint:gosec // length is checked against math.MaxInt32 above
	dataLen := int32(len(data))

	size, err := windows.MultiByteToWideChar(codePage, 0, &data[0], dataLen, nil, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to measure decoded command output")
	}

	if size <= 0 {
		return nil, errEmptyDecodeResult
	}

	decoded := make([]uint16, size)

	_, err = windows.MultiByteToWideChar(codePage, 0, &data[0], dataLen, &decoded[0], size)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode command output")
	}

	return []byte(string(utf16.Decode(decoded))), nil
}
