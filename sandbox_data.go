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
	ID            string                     `yaml:"id"`
	Spec          string                     `yaml:"spec"`
	Worktree      map[string]SandboxWorktree `yaml:"worktree,omitempty"`
	Mounts        map[string]SandboxMount    `yaml:"mounts,omitempty"`
	HarnessMounts map[string]SandboxMount    `yaml:"harness_mounts,omitempty"`
	Leases        map[string]leaseData       `yaml:"leases,omitempty"`
	CreatedAt     time.Time                  `yaml:"created_at"`
	UpdatedAt     time.Time                  `yaml:"updated_at"`
}

type leaseData struct {
	ID        string    `yaml:"id"`
	Path      string    `yaml:"path"`
	WorkDir   string    `yaml:"work_dir"`
	Status    string    `yaml:"status"`
	ExpiresAt time.Time `yaml:"expires_at"`
}

const sandboxType = "sandbox"
const sandboxDataFileName = "data.yml"

const leaseStatusInit = "init"
const leaseStatusRunning = "running"

func (s *sandboxData) save(ctx context.Context, dir string) error {
	s.UpdatedAt = time.Now()
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
