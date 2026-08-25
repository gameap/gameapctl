//go:build !windows

package oscore

import (
	"io"

	"github.com/pkg/errors"
)

// OutputDecoder writes command output as is. Code page conversion is needed on Windows only.
type OutputDecoder struct {
	writer io.Writer
}

// NewOutputDecoder wraps writer with a decoder of the console output code page.
func NewOutputDecoder(writer io.Writer) *OutputDecoder {
	return &OutputDecoder{writer: writer}
}

// Write writes p to the underlying writer.
func (d *OutputDecoder) Write(p []byte) (int, error) {
	n, err := d.writer.Write(p)

	return n, errors.Wrap(err, "failed to write command output")
}

// Flush does nothing, the output is never buffered.
func (d *OutputDecoder) Flush() {}
