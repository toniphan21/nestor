package nestor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
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
const DefaultLeaseInitDuration = time.Minute
const DefaultLeaseExtendDuration = 15 * time.Minute

type SandboxSpecLease struct {
	InitDuration        *time.Duration `yaml:"init_duration,omitempty"`
	InteractiveDuration *time.Duration `yaml:"interactive_duration,omitempty"`
	ExtendDuration      *time.Duration `yaml:"extend_duration,omitempty"`
}

type stateScope string

const StateScopeInstance stateScope = "instance"
const StateScopeShared stateScope = "shared"

type SandboxSpec struct {
	Name         string             `yaml:"-"`
	Harness      harness            `yaml:"harness"`
	Profile      string             `yaml:"profile,omitempty"`
	MCPs         []string           `yaml:"mcp,omitempty"`
	Target       string             `yaml:"target"`
	MaxInstances int                `yaml:"max_instances"`
	StateScope   stateScope         `yaml:"state_scope,omitempty"`
	Lease        *SandboxSpecLease  `yaml:"lease,omitempty"`
	Mounts       []SandboxSpecMount `yaml:"mounts"`
	Env          map[string]string  `yaml:"env,omitempty"`

	profileInYaml *string `yaml:"-"`
	targetInYaml  *string `yaml:"-"`
}

func (s *SandboxSpec) Validate(runtime Runtime) error {
	_, p, err := s.findHarnessAndProfile(runtime)
	if err != nil {
		return err
	}

	if strings.TrimSpace(s.Target) == "" {
		s.Target = p.DefaultTarget
	}

	if s.MaxInstances <= 0 {
		s.MaxInstances = 1
	}

	if s.StateScope == "" {
		s.StateScope = StateScopeShared
	}
	// TODO: continue
	return nil
}

func (s *SandboxSpec) Resolve(path string) (*SandboxSpecMount, string) {
	for _, m := range s.Mounts {
		rel, err := filepath.Rel(m.Path, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return &m, rel
	}
	return nil, ""
}

func (s *SandboxSpec) LeaseInitDuration() time.Duration {
	if s.Lease == nil || s.Lease.InitDuration == nil {
		return DefaultLeaseInitDuration
	}
	return *s.Lease.InitDuration
}

func (s *SandboxSpec) LeaseExtendDuration() time.Duration {
	if s.Lease == nil || s.Lease.ExtendDuration == nil {
		return DefaultLeaseExtendDuration
	}
	return *s.Lease.ExtendDuration
}

func (s *SandboxSpec) ProfileInYaml() *string {
	return s.profileInYaml
}

func (s *SandboxSpec) TargetInYaml() *string {
	return s.targetInYaml
}

func (s *SandboxSpec) findHarnessAndProfile(runtime Runtime) (Harness, Profile, error) {
	h, have := runtime.Registry.Harness(string(s.Harness))
	if !have {
		return nil, Profile{}, fmt.Errorf("%w: harness %q", ErrNotFound, s.Harness)
	}

	pn := s.Profile
	if pn == "" {
		pn = string(s.Harness)
	}
	p, have := runtime.Registry.Profile(pn)
	if !have {
		return nil, Profile{}, fmt.Errorf("%w: profile %q", ErrNotFound, h.Name())
	}

	po := make(map[string]string)
	if p.Options != nil {
		for k, v := range p.Options {
			po[k] = v
		}
	}

	defaultOptions := h.DefaultOptions(runtime)
	for k, v := range defaultOptions {
		if _, have = po[k]; !have {
			po[k] = v
		}
	}

	p.Options = po
	return h, p, nil
}
