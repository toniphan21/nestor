package nestor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/xid"
	"nhatp.com/go/nestor/infra/fs"
)

const DefaultStopContainerTimeout = 2 * time.Second
const DefaultLeaseInitDuration = time.Minute
const DefaultLeaseExtendDuration = 15 * time.Minute
const SandboxRunsTargetPath = "/sandbox/runs"

type Sandbox interface {
	ID() string
	Runtime() Runtime
	Dir(elem ...string) string
	Tag() string
	Spec() SandboxSpec
	Container() string
	Harness() Harness
	Profile() Profile
	Worktrees() []SandboxWorktree
	Paths() []string
	Mounts() []SandboxMount
	Leases() []*Lease
	CreatedAt() time.Time
	UpdatedAt() time.Time

	IsRunning(ctx context.Context) bool
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Sync(ctx context.Context) error
	Delete(ctx context.Context) error

	Acquire(ctx context.Context, path string) (*Lease, error)
}

type Syncer interface {
	Sync(ctx context.Context, sandbox Sandbox) error
}

type SandboxWorktree struct {
	ID            string `yaml:"id"`
	Repository    string `yaml:"repository"`
	Dir           string `yaml:"dir"`
	InitialBranch string `yaml:"initial_branch"`
}

type SandboxMount struct {
	Host     string `yaml:"host"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only"`
}

func makeSandboxImpl(data *sandboxData, runtime Runtime, spec SandboxSpec) (*sandboxImpl, error) {
	h, p, err := spec.findHarnessAndProfile(runtime)
	if err != nil {
		return nil, err
	}

	log := runtime.Logger.With("layer", "sandbox")
	rt := runtime
	rt.Logger = log

	sb := &sandboxImpl{
		data:     data,
		runtime:  rt,
		spec:     spec,
		harness:  h,
		profile:  p,
		git:      rt.newGitFunc(log),
		docker:   rt.newDockerFunc(log),
		log:      log,
		leaseLog: runtime.Logger.With("layer", "lease"),
	}
	return sb, nil
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

	// TODO: validate, the file can be modified manually so we need to validate again
	return makeSandboxImpl(data, runtime, spec)
}

func newSandbox(ctx context.Context, runtime Runtime, spec SandboxSpec) (Sandbox, error) {
	if err := fs.MkdirAll(runtime.Platform.SandboxDir()); err != nil {
		return nil, err
	}

	taken, err := fs.ListDirs(runtime.Platform.SandboxDir())
	if err != nil {
		return nil, err
	}

	id, err := runtime.Template.MakeSandboxID(taken)
	if err != nil {
		return nil, err
	}
	data := &sandboxData{
		ID:        id,
		Spec:      spec.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sandbox, err := makeSandboxImpl(data, runtime, spec)
	if err != nil {
		return nil, err
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
	data     *sandboxData
	runtime  Runtime
	spec     SandboxSpec
	harness  Harness
	profile  Profile
	git      Git
	docker   Docker
	log      *slog.Logger
	leaseLog *slog.Logger
}

func (s *sandboxImpl) ID() string {
	return s.data.ID
}

func (s *sandboxImpl) Runtime() Runtime {
	return s.runtime
}

func (s *sandboxImpl) Dir(elem ...string) string {
	b := s.runtime.Platform.SandboxDir(s.ID())
	if len(elem) == 0 {
		return b
	}

	args := []string{s.ID()}
	args = append(args, elem...)
	return s.runtime.Platform.SandboxDir(args...)
}

func (s *sandboxImpl) Tag() string {
	return s.runtime.Template.MakeSandboxTag(s.spec.Name)
}

func (s *sandboxImpl) Spec() SandboxSpec {
	return s.spec
}

func (s *sandboxImpl) Container() string {
	return s.runtime.Template.MakeSandboxContainer(s.ID())
}

func (s *sandboxImpl) Harness() Harness {
	return s.harness
}

func (s *sandboxImpl) Profile() Profile {
	return s.profile
}

func (s *sandboxImpl) Worktrees() []SandboxWorktree {
	var out []SandboxWorktree
	for _, wt := range s.data.Worktree {
		out = append(out, wt)
	}
	return out
}

func (s *sandboxImpl) Mounts() []SandboxMount {
	var out []SandboxMount
	for _, v := range s.data.Mounts {
		out = append(out, v)
	}
	return out
}

func (s *sandboxImpl) Paths() []string {
	var out []string
	for h, _ := range s.data.Mounts {
		out = append(out, h)
	}
	return out
}

func (s *sandboxImpl) Leases() []*Lease {
	var out []*Lease
	if s.data.Leases == nil {
		s.data.Leases = make(map[string]leaseData)
	}

	var save bool
	for _, ld := range s.data.Leases {
		if ld.ExpiresAt.Before(time.Now()) {
			delete(s.data.Leases, ld.Path)
			save = true
			continue
		}

		out = append(out, &Lease{data: ld, sandbox: s, log: s.leaseLog})
	}

	if save {
		_ = s.save(context.Background())
	}
	return out
}

func (s *sandboxImpl) CreatedAt() time.Time {
	return s.data.CreatedAt
}

func (s *sandboxImpl) UpdatedAt() time.Time {
	return s.data.UpdatedAt
}

func (s *sandboxImpl) IsRunning(ctx context.Context) bool {
	return s.docker.IsRunning(ctx, s.Container())
}

func (s *sandboxImpl) Start(ctx context.Context) error {
	if err := s.Sync(ctx); err != nil {
		return err
	}

	image := s.runtime.Template.MakeSandboxTag(s.spec.Name)
	container := s.runtime.Template.MakeSandboxContainer(s.ID())
	options := DockerRunOption{}

	// mounts from spec
	for _, v := range s.data.Mounts {
		options.Mounts = append(options.Mounts, DockerMount{
			Source:   v.Host,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}

	// mounts from harness
	for _, v := range s.data.HarnessMounts {
		options.Mounts = append(options.Mounts, DockerMount{
			Source:   v.Host,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}

	// mounts for the sandbox
	runsPath := s.Dir("runs")
	if err := fs.MkdirAll(runsPath); err != nil {
		return fmt.Errorf("nestor: cannot make sandbox/runs dir: %w", err)
	}

	options.Mounts = append(options.Mounts, DockerMount{
		Source: runsPath,
		Target: SandboxRunsTargetPath,
	})

	var env = make(map[string]string)
	for k, v := range s.spec.Env {
		env[k] = v
	}
	for k, v := range s.harness.Env(s) {
		env[k] = v
	}
	if len(env) > 0 {
		options.Env = env
	}

	_, err := s.docker.Run(ctx, image, container, options)

	return err
}

func (s *sandboxImpl) Stop(ctx context.Context) error {
	return s.docker.Stop(ctx, s.Container(), DefaultStopContainerTimeout)
}

func (s *sandboxImpl) Sync(ctx context.Context) error {
	for _, v := range s.harness.Syncers(s) {
		if err := v.Sync(ctx, s); err != nil {
			return err
		}
	}

	if err := s.collectMounts(ctx); err != nil {
		return err
	}
	return nil
}

func (s *sandboxImpl) Delete(ctx context.Context) error {
	s.log.Debug("Delete sandbox", slog.String("dir", s.ID()))
	if !fs.HasDir(s.Dir()) {
		s.log.Debug("Delete sandbox: dir not found, nothing to do", slog.String("dir", s.Dir()))
		return nil
	}

	if s.IsRunning(ctx) {
		return fmt.Errorf("%w: sandbox %q is running", ErrNotAllowed, s.ID())
	}

	if err := s.clear(ctx); err != nil {
		return err
	}

	s.log.Debug("Delete sandbox done", slog.String("dir", s.Dir()))
	return nil
}

func (s *sandboxImpl) Acquire(ctx context.Context, path string) (*Lease, error) {
	workDir, ok := s.resolveWorkDir(path)
	if !ok {
		return nil, fmt.Errorf("%w: %q is not under any mount", ErrNotAllowed, path)
	}

	if s.data.Leases == nil {
		s.data.Leases = make(map[string]leaseData)
	}

	ld, ok := s.data.Leases[path]
	if !ok || ld.ExpiresAt.Before(time.Now()) {
		return s.newLease(ctx, path, workDir)
	}
	return nil, fmt.Errorf("%w: lease on path %q is already acquired", ErrNotAvailable, path)
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
			_ = s.git.RemoveWorktree(ctx, v.Repository, v.Dir, v.InitialBranch)
		}
	}

	for _, m := range s.spec.Mounts {
		if m.Type == MountTypeGitWorktree {
			id := s.runtime.Template.MakeWorktreeID(m.Path)
			wtDir := filepath.Join(s.Dir(), id)

			wt := SandboxWorktree{
				ID:            id,
				Dir:           wtDir,
				Repository:    m.Path,
				InitialBranch: s.runtime.Template.MakeInitialBranch(s.ID(), wtDir),
			}

			err := s.git.AddWorktree(ctx, wt.Repository, wt.Dir, wt.InitialBranch)
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
	_ = s.git.RemoveWorktree(ctx, v.Repository, v.Dir, v.InitialBranch)
	delete(s.data.Worktree, s.ID())
	return s.save(ctx)
}

func (s *sandboxImpl) removeAllWorktrees(ctx context.Context) error {
	for _, v := range s.data.Worktree {
		_ = s.git.RemoveWorktree(ctx, v.Repository, v.Dir, v.InitialBranch)
	}
	s.data.Worktree = nil
	return s.save(ctx)
}

func (s *sandboxImpl) collectMounts(ctx context.Context) error {
	hm, err := s.harness.Mounts(s)
	if err != nil {
		return err
	}

	mounts := make(map[string]SandboxMount)
	for _, m := range s.spec.Mounts {
		target, err := m.Target()
		if err != nil {
			return err
		}
		mounts[m.Path] = SandboxMount{Host: m.Path, Target: target, ReadOnly: m.ReadOnly}

		if m.Type == MountTypeGitWorktree {
			for _, wt := range s.data.Worktree {
				if wt.Repository != m.Path {
					continue
				}
				mounts[wt.Dir] = SandboxMount{Host: wt.Dir, Target: wt.Dir}
			}
		}
	}

	s.data.Mounts = mounts
	s.data.HarnessMounts = hm
	return s.save(ctx)
}

func (s *sandboxImpl) newLease(ctx context.Context, path, workDir string) (*Lease, error) {
	ld := leaseData{
		ID:        xid.New().String(),
		Path:      path,
		WorkDir:   workDir,
		ExpiresAt: time.Now().Add(DefaultLeaseInitDuration),
		CreatedAt: time.Now(),
	}

	if s.data.Leases == nil {
		s.data.Leases = make(map[string]leaseData)
	}
	s.data.Leases[path] = ld

	if err := s.save(ctx); err != nil {
		return nil, err
	}
	return &Lease{data: ld, sandbox: s, log: s.leaseLog}, nil
}

func (s *sandboxImpl) resolveWorkDir(path string) (string, bool) {
	for _, m := range s.Mounts() {
		rel, err := filepath.Rel(m.Host, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return filepath.Join(m.Target, rel), true
	}
	return "", false
}
