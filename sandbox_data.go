package nestor

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
	"nhatp.com/go/nestor/infra/fs"
)

type sandboxData struct {
	ID        string                     `yaml:"id"`
	Spec      string                     `yaml:"spec"`
	Worktree  map[string]SandboxWorktree `yaml:"worktree,omitempty"`
	Mounted   map[string]string          `yaml:"mounted,omitempty"`
	CreatedAt time.Time                  `yaml:"created_at"`
	UpdatedAt time.Time                  `yaml:"updated_at"`
}

const sandboxType = "sandbox"
const sandboxDataFileName = "data.yml"

func (s *sandboxData) save(ctx context.Context, dir string) error {
	file := ymlFile[*sandboxData]{
		Version: "1",
		Type:    "sandbox",
		Data:    s,
	}

	err := fs.MkdirAll(dir)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	return fs.AtomicWriteFile(filepath.Join(dir, sandboxDataFileName), b)
}

func readSandboxData(dir string) (*sandboxData, error) {
	yml, err := fs.AtomicReadFile(filepath.Join(dir, sandboxDataFileName))
	if err != nil {
		return nil, fmt.Errorf("cannot read %q in %q: %w", sandboxDataFileName, dir, err)
	}

	return parseYML(bytes.NewBuffer(yml), sandboxType, func(file *ymlFile[*sandboxData]) (*sandboxData, error) {
		if file == nil {
			return nil, nil
		}
		return file.Data, nil
	})
}
