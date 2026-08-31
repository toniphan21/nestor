package nestor

import (
	"embed"
	"path/filepath"

	"nhatp.com/go/nestor/infra/fs"
)

//go:embed all:assets/claude.dockerfile
//go:embed assets/profile.yml
//go:embed assets/sandbox.yml
var builtin embed.FS

type embedAsset struct {
	dst  string
	file bool
}

type embedAssets map[string]embedAsset

func (e embedAssets) save(target string) (bool, error) {
	if fs.HasDir(target) {
		return true, nil
	}

	for name, v := range e {
		if v.file {
			if err := fs.CopyFileFS(builtin, name, filepath.Join(target, v.dst)); err != nil {
				return false, err
			}
			continue
		}

		if err := fs.CopyDirFS(builtin, name, filepath.Join(target, v.dst)); err != nil {
			return false, err
		}
	}
	return false, nil
}
