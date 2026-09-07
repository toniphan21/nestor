package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func AtomicReadFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %#v: %w", path, err)
	}
	return b, nil
}

func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	// WriteFile is not atomic but Rename is, so write to a tmp file then rename.
	// This ensures ReadFile/WriteFile work together across different programs.
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()

	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func CreateFile(path string) (io.WriteCloser, error) {
	return os.Create(path)
}

func WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func RemoveFile(path string) error {
	return os.Remove(path)
}

// MakePrivate restricts path to owner read/write only (0600).
func MakePrivate(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod %q: %w", path, err)
	}
	return nil
}
