package nestor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"nhatp.com/go/nestor/infra/fs"
)

type SandboxSpecMount struct {
	Type     MountType `yaml:"type"`
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

type MountType string

const (
	MountTypeDirect            = MountType("direct")
	MountTypeGitWorktree       = MountType("git_worktree")
	DefaultLeaseInitDuration   = time.Minute
	DefaultLeaseExtendDuration = 15 * time.Minute
)

type SandboxSpecLease struct {
	InitDuration        *time.Duration `yaml:"init_duration,omitempty"`
	InteractiveDuration *time.Duration `yaml:"interactive_duration,omitempty"`
	ExtendDuration      *time.Duration `yaml:"extend_duration,omitempty"`
}

type SandboxSpec struct {
	Name         string             `yaml:"-"`
	Harness      harness            `yaml:"harness"`
	Profile      string             `yaml:"profile,omitempty"`
	Target       string             `yaml:"target"`
	MaxInstances int                `yaml:"max_instances"`
	Mounts       []SandboxSpecMount `yaml:"mounts"`
	MCPs         []string           `yaml:"mcp,omitempty"`
	Lease        *SandboxSpecLease  `yaml:"lease,omitempty"`
	Env          map[string]string  `yaml:"env,omitempty"`

	profileInYaml *string `yaml:"-"`
	targetInYaml  *string `yaml:"-"`
}

func (s *SandboxSpec) Validate(runtime Runtime) error {
	_, p, _, err := s.Resolve(runtime)
	if err != nil {
		return err
	}

	if strings.TrimSpace(s.Target) == "" {
		s.Target = p.DefaultTarget
	}

	if s.MaxInstances <= 0 {
		s.MaxInstances = 1
	}

	if len(s.MCPs) > 0 {
		for _, v := range s.MCPs {
			mcp, ok := runtime.Registry.MCP(v)
			if !ok {
				return fmt.Errorf("%w: MCP %q", ErrNotFound, mcp)
			}
		}
	}
	// TODO: continue
	return nil
}

func (s *SandboxSpec) ResolveMountedPath(path string) (*SandboxSpecMount, string) {
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

func (s *SandboxSpec) DockerImageName(template Template) string {
	return template.MakeSandboxTag(s.Name, s.Target)
}

func (s *SandboxSpec) Paths() []string {
	var paths []string
	for _, v := range s.Mounts {
		paths = append(paths, v.Path)
	}
	return paths
}

func (s *SandboxSpec) Build(ctx context.Context, runtime Runtime, docker Docker, opt DockerBuildOption) error {
	_, _, rp, err := s.Resolve(runtime)
	if err != nil {
		return err
	}

	dockerfile := rp.Dockerfile()
	opt.Dockerfile = dockerfile
	opt.Target = s.Target
	opt.Tag = s.DockerImageName(runtime.Template)

	_, err = docker.Build(ctx, filepath.Dir(dockerfile), opt)
	return err
}

func (s *SandboxSpec) Resolve(runtime Runtime) (Harness, Profile, ResolvedProfile, error) {
	h, have := runtime.Registry.Harness(string(s.Harness))
	if !have {
		return nil, Profile{}, nil, fmt.Errorf("%w: harness %q", ErrNotFound, s.Harness)
	}

	p, have := runtime.Registry.Profile(s.Profile)
	if !have {
		return nil, Profile{}, nil, fmt.Errorf("%w: profile %q", ErrNotFound, h.Name())
	}

	rp, err := h.ResolveProfile(runtime, p)
	if err != nil {
		return nil, Profile{}, nil, err
	}
	return h, p, rp, nil
}

func (s *SandboxSpec) ListSandboxes(runtime Runtime) ([]Sandbox, error) {
	var result []Sandbox
	dirs, err := fs.ListDirs(runtime.Platform.SandboxDir())
	if err != nil {
		return nil, err
	}

	for _, v := range dirs {
		dir := runtime.Platform.SandboxDir(v)
		sb, err := parseSandbox(runtime, dir)
		if err != nil {
			// when sandbox spec not found, it maybe deleted - nothing to warn
			if errors.Is(err, ErrNotFound) {
				continue
			}

			runtime.Logger.Warn("cannot read sandbox", slog.String("dir", dir))
			continue
		}

		if sb.Spec().Name == s.Name {
			result = append(result, sb)
		}
	}
	return result, nil
}
