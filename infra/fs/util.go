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

// MergeDir copies the tree rooted at src onto dst. Files present in src
// replace their counterparts in dst; files present only in dst are left
// untouched. Missing directories are created.
func MergeDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		fi, err := d.Info()
		if err != nil {
			return err
		}

		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, fi.Mode().Perm()); err != nil {
				return fmt.Errorf("merge %q: %w", rel, err)
			}
			return nil
		case fi.Mode().IsRegular():
			if err := copyFile(path, target, fi.Mode().Perm()); err != nil {
				return fmt.Errorf("merge %q: %w", rel, err)
			}
			return nil
		default:
			return nil // skip symlinks, sockets, devices
		}
	})
}

// CopyFile copies the regular file src to dst, overwriting it and matching
// src's permissions.
func CopyFile(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	return copyFile(src, dst, fi.Mode().Perm())
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// mode above only applies on creation; force it for existing files too.
	if err := out.Chmod(mode); err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	return err
}
