package nestor

import (
	"fmt"
	"path/filepath"
	"strings"
)

type SandboxSpec struct {
	Name         string         `yaml:"-"`
	Harness      harness        `yaml:"harness"`
	Profile      string         `yaml:"profile,omitempty"`
	Target       string         `yaml:"target"`
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

func (s *SandboxSpec) Tag(template string) string {
	var vars = map[string]string{
		"[sandbox-name]": s.Name,
		"[sandboxName]":  s.Name,
		"$sandboxName":   s.Name,
		"$name":          s.Name,
	}

	var out = template
	for k, v := range vars {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func (s *SandboxSpec) ResolveHarness(ctx Context) (*ResolvedHarness, error) {
	h, have := ctx.Registry().Harness(string(s.Harness))
	if !have {
		return nil, fmt.Errorf("%w: harness %q", ErrNotFound, s.Harness)
	}

	p, have := ctx.Registry().Profile(h.Name())
	if have {
		h.FillProfileDefaultValues(ctx, &p)
		return &ResolvedHarness{Harness: h, Profile: p}, nil
	}

	return nil, fmt.Errorf("%w: profile %q", ErrNotFound, h.Name())
}

type ResolvedHarness struct {
	Harness Harness
	Profile Profile
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
