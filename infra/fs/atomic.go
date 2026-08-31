package fs

import (
	"fmt"
	"os"
)

func AtomicReadFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %#v: %w", path, err)
	}
	return b, nil
}

func AtomicWriteFile(path string, data []byte) error {
	// WriteFile is not atomic but Rename is, so write to a tmp file then rename.
	// This ensures ReadFile/WriteFile work together across different programs.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func RemoveFile(path string) error {
	return os.Remove(path)
}
