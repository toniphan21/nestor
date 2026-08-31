package fs

import (
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
)

func MkdirAll(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot make dir: %w", err)
	}
	return nil
}

// HasDir reports whether path exists and is a directory.
// Any error other than non-existence (permission denied, I/O error)
// is reported as false.
func HasDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// HasFile reports whether path exists and is a regular file.
func HasFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// CopyFileFS copies name from src to the local path dst, overwriting it.
func CopyFileFS(src iofs.FS, name, dst string) error {
	in, err := src.Open(name)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// CopyDirFS copies the tree rooted at root in src to the local directory dst.
func CopyDirFS(src iofs.FS, root, dst string) error {
	sub, err := iofs.Sub(src, root)
	if err != nil {
		return err
	}

	return iofs.WalkDir(sub, ".", func(name string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(dst, filepath.FromSlash(name))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return CopyFileFS(sub, name, target)
	})
}
