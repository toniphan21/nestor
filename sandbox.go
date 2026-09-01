package nestor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"nhatp.com/go/nestor/infra/fs"
	"nhatp.com/go/nestor/infra/git"
)

type Sandbox interface {
	ID() string
	Runtime() Runtime
	Dir() string
	Tag() string
	Spec() SandboxSpec
	Container() string
	Harness() Harness
	Profile() Profile
	Worktrees() []SandboxWorktree
	Mounts() []string
	Leases() []Lease
	CreatedAt() time.Time
	UpdatedAt() time.Time

	IsRunning(ctx context.Context) bool
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Sync(ctx context.Context) error
	Delete(ctx context.Context) error
}

type Syncer interface {
	fmt.Stringer

	Sync(ctx context.Context, sandbox Sandbox) error
}

type SandboxWorktree struct {
	ID            string `yaml:"id"`
	Repository    string `yaml:"repository"`
	Dir           string `yaml:"dir"`
	InitialBranch string `yaml:"initialBranch"`
}

func parseSandbox(runtime Runtime, dir string) (Sandbox, error) {
	data, err := readSandboxData(dir)
	if err != nil {
		return nil, err
	}

	spec, have := runtime.Registry.SandboxSpec(data.Spec)
	if !have {
		return nil, fmt.Errorf("%w: sandbox %q", ErrNotFound, data.Spec)
	}
	h, p, err := spec.resolveHarness(runtime)

	sb := &sandboxImpl{
		data:    data,
		runtime: runtime,
		spec:    spec,
		harness: h,
		profile: p,
	}

	// TODO: validate, the file can be modified manually so we need to validate again
	return sb, nil
}

func newSandbox(ctx context.Context, runtime Runtime, spec SandboxSpec) (Sandbox, error) {
	h, p, err := spec.resolveHarness(runtime)
	if err = fs.MkdirAll(runtime.Platform.SandboxDir()); err != nil {
		return nil, err
	}

	taken, err := fs.ListDirs(runtime.Platform.SandboxDir())
	if err != nil {
		return nil, err
	}

	id, err := runtime.Template.makeSandboxID(taken)
	if err != nil {
		return nil, err
	}
	data := &sandboxData{
		ID:        id,
		Spec:      spec.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	sandbox := &sandboxImpl{
		data:    data,
		runtime: runtime,
		spec:    spec,
		harness: h,
		profile: p,
	}

	if err = sandbox.save(ctx); err != nil {
		return nil, err
	}

	if err = sandbox.makeWorktree(ctx); err != nil {
		_ = sandbox.clear(ctx)
		return nil, err
	}

	if err = sandbox.Sync(ctx); err != nil {
		_ = sandbox.clear(ctx)
		return nil, err
	}
	return sandbox, nil
}

type sandboxImpl struct {
	data    *sandboxData
	runtime Runtime
	spec    SandboxSpec
	harness Harness
	profile Profile
}

func (s *sandboxImpl) ID() string {
	return s.data.ID
}

func (s *sandboxImpl) Runtime() Runtime {
	return s.runtime
}

func (s *sandboxImpl) Dir() string {
	return s.runtime.Platform.SandboxDir(s.ID())
}

func (s *sandboxImpl) Tag() string {
	return s.runtime.Template.makeSandboxTag(s.spec.Name)
}

func (s *sandboxImpl) Spec() SandboxSpec {
	return s.spec
}

func (s *sandboxImpl) Container() string {
	return s.runtime.Template.makeSandboxContainer(s.ID())
}

func (s *sandboxImpl) Harness() Harness {
	return s.harness
}

func (s *sandboxImpl) Profile() Profile {
	return s.profile
}

func (s *sandboxImpl) Worktrees() []SandboxWorktree {
	panic("implement me")
}

func (s *sandboxImpl) Mounts() []string {
	panic("implement me")
}

func (s *sandboxImpl) Leases() []Lease {
	panic("implement me")
}

func (s *sandboxImpl) CreatedAt() time.Time {
	panic("implement me")
}

func (s *sandboxImpl) UpdatedAt() time.Time {
	panic("implement me")
}

func (s *sandboxImpl) IsRunning(ctx context.Context) bool {
	panic("implement me")
}

func (s *sandboxImpl) Start(ctx context.Context) error {
	panic("implement me")
}

func (s *sandboxImpl) Stop(ctx context.Context) error {
	panic("implement me")
}

func (s *sandboxImpl) Sync(ctx context.Context) error {
	syncers := s.harness.Syncers(s.runtime, s.profile)
	for _, v := range syncers {
		if err := v.Sync(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (s *sandboxImpl) Delete(ctx context.Context) error {
	log := s.runtime.Logger
	log.Debug("Delete sandbox", slog.String("dir", s.ID()))
	if !fs.HasDir(s.Dir()) {
		log.Debug("Delete sandbox: dir not found, nothing to do", slog.String("dir", s.Dir()))
		return nil
	}

	if s.IsRunning(ctx) {
		return fmt.Errorf("%w: sandbox %q is running", ErrNotAllowed, s.ID())
	}

	if err := s.clear(ctx); err != nil {
		return err
	}

	log.Debug("Delete sandbox done", slog.String("dir", s.Dir()))
	return nil
}

func (s *sandboxImpl) clear(ctx context.Context) error {
	if err := s.removeAllWorktrees(ctx); err != nil {
		return fmt.Errorf("nestor: cannot remove sandbox worktrees: %w", err)
	}

	if err := fs.RemoveDir(s.Dir()); err != nil {
		return fmt.Errorf("nestor: cannot remove sandbox dir: %w", err)
	}
	return nil
}

func (s *sandboxImpl) save(ctx context.Context) error {
	return s.data.save(ctx, s.Dir())
}

func (s *sandboxImpl) makeWorktree(ctx context.Context) error {
	worktrees := make(map[string]SandboxWorktree)
	cleanUp := func() {
		for _, v := range s.data.Worktree {
			_ = git.RemoveWorktree(v.Repository, v.Dir, v.InitialBranch, s.runtime.Logger)
		}
	}

	for _, m := range s.spec.Mounts {
		if m.Type == MountTypeGitWorktree {
			id := s.runtime.Template.makeWorktreeID(m.Path)
			wtDir := filepath.Join(s.Dir(), id)

			wt := SandboxWorktree{
				ID:            id,
				Dir:           wtDir,
				Repository:    m.Path,
				InitialBranch: s.runtime.Template.makeInitialBranch(s.ID(), wtDir),
			}

			err := git.AddWorktree(wt.Repository, wt.Dir, wt.InitialBranch, s.runtime.Logger)
			if err != nil {
				cleanUp()
				return fmt.Errorf("cannot create worktree for %s: %w", m.Path, err)
			}
			worktrees[wt.ID] = wt
		}
	}

	s.data.Worktree = worktrees
	return s.save(ctx)
}

func (s *sandboxImpl) removeWorktree(ctx context.Context) error {
	v, ok := s.data.Worktree[s.ID()]
	if !ok {
		return nil
	}
	_ = git.RemoveWorktree(v.Repository, v.Dir, v.InitialBranch, s.runtime.Logger)
	delete(s.data.Worktree, s.ID())
	return s.save(ctx)
}

func (s *sandboxImpl) removeAllWorktrees(ctx context.Context) error {
	for _, v := range s.data.Worktree {
		_ = git.RemoveWorktree(v.Repository, v.Dir, v.InitialBranch, s.runtime.Logger)
	}
	s.data.Worktree = nil
	return s.save(ctx)
}
