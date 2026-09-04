package nestor

import (
	"fmt"
	"strings"
)

type SandboxSpecMount struct {
	Type     mountType `yaml:"type"`
	Path     string    `yaml:"path"`
	At       string    `yaml:"at,omitempty"`
	ReadOnly bool      `yaml:"read_only"`
}

func (m *SandboxSpecMount) Target() (string, error) {
	switch m.Type {
	case MountTypeDirect:
		at := m.At
		if at == "" {
			at = m.Path
		}
		return at, nil

	case MountTypeGitWorktree:
		if m.ReadOnly {
			return "", fmt.Errorf("%w: git_worktree mount %q cannot be read-only", ErrNotAllowed, m.Path)
		}
		if m.At != "" && m.At != m.Path {
			return "", fmt.Errorf("%w: git_worktree mount %q cannot set at=%q", ErrNotAllowed, m.Path, m.At)
		}
		return m.Path, nil

	default:
		return "", fmt.Errorf("%w: mount %q has type %q", ErrNotSupported, m.Path, m.Type)
	}
}

type mountType string

const MountTypeDirect = mountType("direct")
const MountTypeGitWorktree = mountType("git_worktree")

type SandboxSpec struct {
	Name         string             `yaml:"-"`
	Harness      harness            `yaml:"harness"`
	Profile      string             `yaml:"profile,omitempty"`
	Target       string             `yaml:"target"`
	MaxInstances int                `yaml:"max_instances"`
	Mounts       []SandboxSpecMount `yaml:"mounts"`
	Env          map[string]string  `yaml:"env,omitempty"`
}

func (s *SandboxSpec) Validate(runtime Runtime) error {
	_, p, err := s.findHarnessAndProfile(runtime)
	if err != nil {
		return err
	}

	if strings.TrimSpace(s.Target) == "" {
		s.Target = p.DefaultTarget
	}

	// TODO: continue
	return nil
}

func (s *SandboxSpec) findHarnessAndProfile(runtime Runtime) (Harness, Profile, error) {
	h, have := runtime.Registry.Harness(string(s.Harness))
	if !have {
		return nil, Profile{}, fmt.Errorf("%w: harness %q", ErrNotFound, s.Harness)
	}

	p, have := runtime.Registry.Profile(h.Name())
	if !have {
		return nil, Profile{}, fmt.Errorf("%w: profile %q", ErrNotFound, h.Name())
	}

	h.FillProfileDefaultValues(runtime, &p)
	return h, p, nil
}
