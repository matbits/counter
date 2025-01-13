package fhandler

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrSourceDir for when source is not a directory.
	ErrSourceDir = errors.New("source is not a directory")
	// ErrDestinationExists for when destination already exists.
	ErrDestinationExists = errors.New("destination already exists")
)

func Rename(src, dst string) error {
	err := os.Rename(src, dst)
	// cross device move
	if err != nil && strings.HasSuffix(err.Error(), "invalid cross-device link") {
		fileInfo, err := os.Stat(src)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			if err := CopyDir(src, dst); err != nil {
				return err
			}

			if err := os.RemoveAll(src); err != nil {
				return err
			}
		} else {
			if err := CopyFile(src, dst); err != nil {
				return err
			}

			if err := os.Remove(src); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage.
func CopyFile(src, dst string) (err error) {
	input, err := os.Open(src)
	if err != nil {
		return err
	}

	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		if e := output.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(output, input)
	if err != nil {
		return err
	}

	err = output.Sync()
	if err != nil {
		return err
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return err
	}

	return nil
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func CopyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !fileInfo.IsDir() {
		return ErrSourceDir
	}

	_, err = os.Stat(dst)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err == nil {
		return ErrDestinationExists
	}

	err = os.MkdirAll(dst, fileInfo.Mode())
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Skip symlinks.
			fsInfo, err := entry.Info()
			if err != nil {
				return err
			}

			if fsInfo.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
