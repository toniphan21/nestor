package nestor

import (
	"path/filepath"
	"strings"
)

type SandboxSpec struct {
	Name         string         `yaml:"-"`
	Harness      harness        `yaml:"harness"`
	Target       string         `yaml:"target"`
	Auth         auth           `yaml:"auth"`
	MaxInstances int            `yaml:"max_instances"`
	Mounts       []SandboxMount `yaml:"mounts"`
}

func (s *SandboxSpec) Validate() error {
	return nil
}

func (s *SandboxSpec) Resolve(workDir string) (string, bool) {
	for _, m := range s.Mounts {
		rel, err := filepath.Rel(m.Path, workDir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return filepath.Join(m.At, rel), true
	}
	return "", false
}

type SandboxMount struct {
	Type     mountType `yaml:"type"`
	Path     string    `yaml:"path"`
	At       string    `yaml:"at,omitempty"`
	ReadOnly bool      `yaml:"read_only"`
}

type auth string

const AuthCredentials = auth("credentials")
const AuthAPIKey = auth("api_key")

type mountType string

const MountTypeDirect = mountType("direct")
const MountTypeGitWorktree = mountType("git_worktree")
