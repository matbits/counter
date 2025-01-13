package fhandler

import (
	"os"
	"strings"
)

func WriteAtomicTmpDir(prefix string, file string, content []byte, permission os.FileMode) error {
	return WriteAtomic(os.TempDir(), prefix, file, content, permission)
}

func WriteAtomic(dir string, prefix string, file string, content []byte, permission os.FileMode) error {
	tmpName, err := writeTmpFile(dir, prefix, content)
	if err != nil {
		return err
	}

	err = os.Chmod(tmpName, permission)
	if err != nil {
		return err
	}

	return Rename(tmpName, file)
}

func WriteAtomicTmp(prefix string, content []byte) (string, error) {
	tmpName, err := writeTmpFile(os.TempDir(), prefix, content)
	if err != nil {
		return "", err
	}

	return tmpName, nil
}

func writeTmpFile(dir string, prefix string, content []byte) (string, error) {
	if !strings.Contains(prefix, "*") {
		prefix = prefix + "_*"
	}

	tmpFile, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return "", err
	}

	defer tmpFile.Close()

	_, err = tmpFile.Write(content)
	if err != nil {
		os.Remove(tmpFile.Name())

		return "", err
	}

	return tmpFile.Name(), nil
}
