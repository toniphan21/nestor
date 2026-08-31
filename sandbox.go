package nestor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"nhatp.com/go/nestor/infra/docker"
	"nhatp.com/go/nestor/infra/fs"
	"nhatp.com/go/nestor/infra/git"
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

type Syncer interface {
	fmt.Stringer

	Sync(sandbox Sandbox, log *slog.Logger) error
}

type Sandbox struct {
	ID        string                     `json:"id"`
	Dir       string                     `json:"dir"`
	Spec      string                     `json:"spec"`
	Tag       string                     `json:"tag"`
	Status    string                     `json:"status"`
	Worktree  map[string]SandboxWorktree `json:"worktree,omitempty"`
	Mounted   map[string]string          `json:"mounted,omitempty"`
	CreatedAt time.Time                  `json:"created_at"`
	UpdatedAt time.Time                  `json:"updated_at"`
}

func (s *Sandbox) IsRunning(ctx Context, logger *slog.Logger) bool {
	template := ctx.Template()
	return docker.IsRunning(template.makeSandboxContainer(s), logger)
}

func (s *Sandbox) makeWorktree(ctx Context, spec SandboxSpec, log *slog.Logger) error {
	worktrees := make(map[string]SandboxWorktree)
	cleanUp := func() {
		for _, v := range s.Worktree {
			_ = git.RemoveWorktree(v.Repository, v.Dir, v.InitialBranch, log)
		}
	}

	for _, m := range spec.Mounts {
		if m.Type == MountTypeGitWorktree {
			template := ctx.Template()
			id := template.makeWorktreeID(m.Path)
			wtDir := filepath.Join(s.Dir, id)

			wt := SandboxWorktree{
				ID:            id,
				Dir:           wtDir,
				Repository:    m.Path,
				InitialBranch: template.makeInitialBranch(s.ID, wtDir),
			}

			err := git.AddWorktree(wt.Repository, wt.Dir, wt.InitialBranch, log)
			if err != nil {
				cleanUp()
				return fmt.Errorf("cannot create worktree for %s: %w", m.Path, err)
			}
			worktrees[wt.ID] = wt
		}
	}

	s.Worktree = worktrees
	return s.save(ctx)
}

func (s *Sandbox) removeWorktree(ctx Context, id string, log *slog.Logger) error {
	v, ok := s.Worktree[id]
	if !ok {
		return nil
	}
	_ = git.RemoveWorktree(v.Repository, v.Dir, v.InitialBranch, log)
	delete(s.Worktree, id)
	return s.save(ctx)
}

func (s *Sandbox) removeAllWorktrees(ctx Context, log *slog.Logger) error {
	for _, v := range s.Worktree {
		_ = git.RemoveWorktree(v.Repository, v.Dir, v.InitialBranch, log)
	}
	s.Worktree = nil
	return s.save(ctx)
}

func (s *Sandbox) save(ctx Context) error {
	s.Dir = ctx.Platform().SandboxDir(s.ID)
	err := fs.MkdirAll(s.Dir)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fs.AtomicWriteFile(filepath.Join(s.Dir, "sandbox.json"), b)
}

func readSandbox(dir string) (Sandbox, error) {
	b, err := fs.AtomicReadFile(filepath.Join(dir, "sandbox.json"))
	if err != nil {
		return Sandbox{}, err
	}
	var s Sandbox
	if err := json.Unmarshal(b, &s); err != nil {
		return Sandbox{}, err
	}
	return s, nil
}

const SandboxStatusStop = "stop"
const SandboxStatusRunning = "running"

type SandboxWorktree struct {
	ID            string `json:"id"`
	Repository    string `json:"repository"`
	Dir           string `json:"dir"`
	InitialBranch string `json:"initialBranch"`
}
