package sendlogs

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log"
	"os"
	"path/filepath"
)

// https://gist.github.com/mimoo/25fc9716e0f1353791f5908f94d6e726
func compress(src string, buf io.Writer) error {
	// tar > gzip > buf
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	// walk through every file in the folder
	err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Println("file was skipped", err)

			return nil
		}

		// generate tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, data)
			data.Close()
			if copyErr != nil {
				return copyErr
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// produce tar
	if err := tw.Close(); err != nil {
		return err
	}
	// produce gzip
	if err := gw.Close(); err != nil {
		return err
	}
	//
	return nil
}
