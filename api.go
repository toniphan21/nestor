package nestor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"nhatp.com/go/nestor/infra/docker"
	"nhatp.com/go/nestor/infra/fs"
)

type API interface {
	Build(ctx context.Context, specs ...string) error

	List(ctx context.Context, specs ...string) ([]Sandbox, error)

	Create(ctx context.Context, spec string) (Sandbox, error)

	Sync(ctx context.Context, spec string) error

	SyncSandbox(ctx context.Context, sandbox Sandbox) error

	Start(ctx context.Context, spec string) error
}

const DefaultSandboxIDLength = 5
const DefaultLogFile = "nestor.log"
const DefaultSandboxTagTemplate = "nestor-[sandboxName]"

func New(options ...Option) (API, error) {
	o := &opts{
		logger:             slog.New(slog.DiscardHandler),
		registry:           newRegistry(),
		sandboxTagTemplate: DefaultSandboxTagTemplate,
		sandboxIDLength:    DefaultSandboxIDLength,
	}

	// apply options
	for _, v := range options {
		v.apply(o)
	}

	if o.platform == nil {
		defaultPlatform, err := DefaultPlatform()
		if err != nil {
			return nil, err
		}
		o.platform = defaultPlatform
	}

	if o.dir == "" {
		o.dir = o.platform.NestorDir()
	}

	if err := fs.MkdirAll(o.dir); err != nil {
		return nil, err
	}

	a := &api{
		dir:                o.dir,
		platform:           o.platform.Clone(o.dir),
		logger:             o.logger,
		registry:           o.registry,
		sandboxTagTemplate: o.sandboxTagTemplate,
		sandboxIDLength:    o.sandboxIDLength,
	}

	// register sandboxSpecs and profiles passed via options
	if len(o.profiles) > 0 {
		for _, v := range o.profiles {
			a.registry.RegisterProfile(v)
		}
	}

	if err := a.init(); err != nil {
		return nil, err
	}
	return a, nil
}

type api struct {
	dir                string
	platform           Platform
	registry           Registry
	logger             *slog.Logger
	sandboxTagTemplate string
	sandboxIDLength    byte
}

func (a *api) makeContext(ctx context.Context) Context {
	return newContext(ctx, a.platform, a.registry)
}

func (a *api) init() error {
	a.logger.Info("Init start", slog.String("dir", a.dir))
	ctx := a.makeContext(context.Background())

	// sandbox.yml is saved from assets for the first time in NESTOR_DIR/sandbox.yml
	sf := a.platform.SandboxYmlFile()
	if !fs.HasFile(sf) {
		if err := fs.CopyFileFS(builtin, "assets/sandbox.yml", sf); err != nil {
			a.logger.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.logger.Info("saved builtin sandbox.yml file", slog.String("path", sf))
	}

	// if there is no sandbox provided, parse from NESTOR_DIR/sandbox.yml
	// for other location the caller need to parse manually add pass via WithSandboxSpecs
	if len(a.registry.SandboxSpecs()) == 0 {
		yml, err := fs.AtomicReadFile(sf)
		if err != nil {
			e := fmt.Errorf("cannot read sandbox.yml file: %w", err)
			a.logger.Error(e.Error(), slog.Any("error", e), slog.String("path", sf))
			return e
		}
		sbs, err := ParseSandboxSpecs(bytes.NewBuffer(yml))
		if err != nil {
			e := fmt.Errorf("cannot parse sandbox.yml file: %w", err)
			a.logger.Error(e.Error(), slog.Any("error", e), slog.String("path", sf))
			return e
		}

		for _, v := range sbs {
			a.registry.RegisterSandboxSpec(v)
		}
		a.logger.Info("loaded sandbox specs", slog.String("path", sf))
	} else {
		a.logger.Info("use sandbox specs from WithSandboxSpecs")
	}

	// profile.yml is saved from assets for the first time in NESTOR_DIR/profile.yml
	pf := a.platform.ProfileYmlFile()
	if !fs.HasFile(pf) {
		if err := fs.CopyFileFS(builtin, "assets/profile.yml", pf); err != nil {
			a.logger.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.logger.Info("saved builtin profile.yml file", slog.String("path", pf))
	}

	// if there is no profiles provided, parse from NESTOR_DIR/profile.yml
	// for other location the caller need to parse manually add pass via WithProfiles
	if len(a.registry.Profiles()) == 0 {
		yml, err := fs.AtomicReadFile(pf)
		if err != nil {
			e := fmt.Errorf("cannot read profile.yml file: %w", err)
			a.logger.Error(e.Error(), slog.Any("error", e), slog.String("path", pf))
			return e
		}
		hss, err := ParseProfiles(bytes.NewBuffer(yml))
		if err != nil {
			e := fmt.Errorf("cannot parse profile.yml file: %w", err)
			a.logger.Error(e.Error(), slog.Any("error", e), slog.String("path", pf))
			return e
		}

		for _, v := range hss {
			a.registry.RegisterProfile(v)
		}
		a.logger.Info("loaded harness specs", slog.String("path", pf))
	} else {
		a.logger.Info("use harness specs from WithProfiles")
	}

	// initialize builtin harnesses
	for _, v := range a.registry.Harnesses() {
		if err := v.Init(ctx, a.dir, a.logger); err != nil {
			return err
		}
	}
	a.logger.Info("builtin harnesses initialized")

	a.logger.Info("Init done", slog.String("dir", a.dir))
	return nil
}

func (a *api) Build(ctx context.Context, specs ...string) error {
	a.logger.Info("Build start", slog.String("dir", a.dir))
	actx := a.makeContext(ctx)

	var selected = make(map[string]bool)
	if len(specs) == 0 {
		for _, v := range a.registry.SandboxSpecs() {
			selected[v.Name] = true
		}
	}

	for _, sandbox := range a.registry.SandboxSpecs() {
		if !selected[sandbox.Name] {
			continue
		}

		r, err := sandbox.ResolveHarness(actx)
		if err != nil {
			return err
		}

		buildPath := filepath.Dir(r.Profile.Dockerfile)
		options := docker.BuildOptions{
			Dockerfile: r.Profile.Dockerfile,
			Target:     sandbox.Target,
			Tag:        sandbox.Tag(a.sandboxTagTemplate),
		}

		if _, err = docker.Build(ctx, buildPath, options, a.logger); err != nil {
			return err
		}
	}

	a.logger.Info("Build done", slog.String("dir", a.dir))
	return nil
}

func (a *api) List(ctx context.Context, specs ...string) ([]Sandbox, error) {
	a.logger.Info("List start", slog.String("dir", a.dir))
	ctx = a.makeContext(ctx)

	if !fs.HasDir(a.platform.SandboxDir()) {
		return nil, nil
	}

	var selected = make(map[string]bool)
	if len(specs) == 0 {
		for _, v := range a.registry.SandboxSpecs() {
			selected[v.Name] = true
		}
	}

	var result []Sandbox
	dirs, err := fs.ListDirs(a.platform.SandboxDir())
	if err != nil {
		return nil, err
	}

	for _, v := range dirs {
		dir := a.platform.SandboxDir(v)
		sb, err := readSandbox(dir)
		if err != nil {
			a.logger.Warn("cannot read sandbox.json", slog.String("dir", dir))
			continue
		}

		if selected[sb.Spec] {
			result = append(result, sb)
		}
	}
	fmt.Println(result)

	a.logger.Info("List done", slog.String("dir", a.dir))
	return result, nil
}

func (a *api) Create(ctx context.Context, spec string) (Sandbox, error) {
	a.logger.Info("Create start", slog.String("dir", a.dir))
	actx := a.makeContext(ctx)
	sandbox := Sandbox{}

	ss, ok := a.registry.SandboxSpec(spec)
	if !ok {
		return sandbox, fmt.Errorf("%w: sandbox %q", ErrNotFound, spec)
	}

	// TODO: validate max_instances

	if err := fs.MkdirAll(a.platform.SandboxDir()); err != nil {
		return sandbox, err
	}
	dirs, err := fs.ListDirs(a.platform.SandboxDir())
	if err != nil {
		return sandbox, err
	}

	id, err := makeID(int(a.sandboxIDLength), dirs)
	if err != nil {
		return sandbox, err
	}

	sandbox.ID = id
	sandbox.Spec = ss.Name
	sandbox.Dir = a.platform.SandboxDir(id)
	sandbox.Tag = ss.Tag(a.sandboxTagTemplate)
	sandbox.Status = SandboxStatusStop
	if err = sandbox.save(actx); err != nil {
		return Sandbox{}, err
	}

	// TODO: init dir, worktree dance

	if err = a.SyncSandbox(ctx, sandbox); err != nil {
		_ = fs.RemoveDir(sandbox.Dir)
		return Sandbox{}, err
	}

	a.logger.Info("Create end", slog.String("dir", a.dir))
	return sandbox, nil
}

func (a *api) Sync(ctx context.Context, spec string) error {
	a.logger.Info("Sync start", slog.String("dir", a.dir))
	actx := a.makeContext(ctx)

	ss, ok := a.registry.SandboxSpec(spec)
	if !ok {
		return fmt.Errorf("%w: sandbox %q", ErrNotFound, spec)
	}

	sbs, err := a.List(ctx, ss.Name)
	if err != nil {
		return err
	}

	for _, v := range sbs {
		if err = a.sync(actx, ss, v); err != nil {
			return err
		}
	}

	a.logger.Info("Sync end", slog.String("dir", a.dir))
	return nil
}

func (a *api) SyncSandbox(ctx context.Context, sandbox Sandbox) error {
	a.logger.Info("SyncSandbox start", slog.String("dir", a.dir))
	actx := a.makeContext(ctx)

	ss, ok := a.registry.SandboxSpec(sandbox.Spec)
	if !ok {
		return fmt.Errorf("%w: sandbox %q", ErrNotFound, sandbox.Spec)
	}

	if err := a.sync(actx, ss, sandbox); err != nil {
		return err
	}

	a.logger.Info("SyncSandbox end", slog.String("dir", a.dir))
	return nil
}

func (a *api) sync(ctx Context, spec SandboxSpec, sandbox Sandbox) error {
	r, err := spec.ResolveHarness(ctx)
	if err != nil {
		return err
	}

	syncers := r.Harness.Syncers(ctx, r.Profile)
	for _, v := range syncers {
		if err = v.Sync(sandbox, a.logger); err != nil {
			return err
		}
	}
	return nil
}

func (a *api) Start(ctx context.Context, sandbox string) error {
	//TODO implement me
	panic("implement me")
}

var _ API = (*api)(nil)
