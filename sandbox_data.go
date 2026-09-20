package nestor

import (
	"bytes"
	"context"
	"errors"
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
	Sessions      map[string][]string        `yaml:"sessions,omitempty"`
	CreatedAt     time.Time                  `yaml:"created_at"`
	UpdatedAt     time.Time                  `yaml:"updated_at"`
}

type leaseData struct {
	ID          string    `yaml:"id"`
	Path        string    `yaml:"path"`
	WorkDir     string    `yaml:"work_dir"`
	HostWorkDir string    `yaml:"host_work_dir"`
	ExpiresAt   time.Time `yaml:"expires_at"`
	CreatedAt   time.Time `yaml:"created_at"`
}

const sandboxType = "sandbox"
const sandboxDataFileName = "data.yml"

func (s *sandboxData) save(ctx context.Context, dir string) error {
	return writeDataFile(dir, sandboxType, sandboxDataFileName, s)
}

func readSandboxData(dir string) (*sandboxData, error) {
	return readDataFile[sandboxData](dir, sandboxType, sandboxDataFileName)
}

const sandboxSharedType = "sandbox-shared"
const sandboxSharedDataFileName = "sandboxes.yml"

type sandboxSharedData struct {
	Sessions map[string][]string
}

func (s *sandboxSharedData) save(ctx context.Context, dir string) error {
	return writeDataFile(dir, sandboxSharedType, sandboxSharedDataFileName, s)
}

func readSandboxSharedData(dir string) (*sandboxSharedData, error) {
	out, err := readDataFile[sandboxSharedData](dir, sandboxSharedType, sandboxSharedDataFileName)
	if err == nil {
		return out, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return &sandboxSharedData{}, nil
	}
	return nil, err
}

func writeDataFile[T any](dir string, typ string, fn string, data *T) error {
	file := ymlFile[*T]{
		Version: "1",
		Type:    typ,
		Data:    data,
	}

	err := fs.MkdirAll(dir)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	return fs.AtomicWriteFile(filepath.Join(dir, fn), b, 0644)
}

func readDataFile[T any](dir string, typ string, fn string) (*T, error) {
	yml, err := fs.AtomicReadFile(filepath.Join(dir, fn))
	if err != nil {
		return nil, fmt.Errorf("cannot read %q in %q: %w", fn, dir, err)
	}

	return parseYML(bytes.NewBuffer(yml), typ, func(file *ymlFile[*T]) (*T, error) {
		if file == nil {
			return nil, nil
		}
		return file.Data, nil
	})
}
