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
}

const DefaultLogFile = "nestor.log"

func New(options ...Option) (API, error) {
	o := &opts{
		logger:   slog.New(slog.DiscardHandler),
		registry: newRegistry(),
		template: DefaultTemplate(),
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
		dir:      o.dir,
		logger:   o.logger,
		platform: o.platform.Clone(o.dir),
		registry: o.registry,
		template: o.template,
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
	dir      string
	platform Platform
	registry Registry
	template Template
	logger   *slog.Logger
}

func (a *api) makeRuntime() Runtime {
	return Runtime{Registry: a.registry, Platform: a.platform, Template: a.template, Logger: a.logger}
}

func (a *api) init() error {
	a.logger.Debug("Init start", slog.String("dir", a.dir))

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
	runtime := a.makeRuntime()
	for _, v := range a.registry.Harnesses() {
		if err := v.Init(runtime); err != nil {
			return err
		}
	}
	a.logger.Debug("builtin harnesses initialized")

	a.logger.Debug("Init done", slog.String("dir", a.dir))
	return nil
}

func (a *api) Build(ctx context.Context, specs ...string) error {
	a.logger.Debug("Build start", slog.String("dir", a.dir))
	runtime := a.makeRuntime()

	var selected = make(map[string]bool)
	if len(specs) == 0 {
		for _, v := range a.registry.SandboxSpecs() {
			selected[v.Name] = true
		}
	}

	for _, spec := range a.registry.SandboxSpecs() {
		if !selected[spec.Name] {
			continue
		}

		_, profile, err := spec.resolveHarness(runtime)
		if err != nil {
			return err
		}

		buildPath := filepath.Dir(profile.Dockerfile)
		options := docker.BuildOptions{
			Dockerfile: profile.Dockerfile,
			Target:     spec.Target,
			Tag:        a.template.makeSandboxTag(spec.Name),
		}

		if _, err = docker.Build(ctx, buildPath, options, a.logger); err != nil {
			return err
		}
	}

	a.logger.Debug("Build done", slog.String("dir", a.dir))
	return nil
}

func (a *api) List(ctx context.Context, specs ...string) ([]Sandbox, error) {
	a.logger.Debug("List start", slog.String("dir", a.dir))
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

	runtime := a.makeRuntime()
	for _, v := range dirs {
		dir := a.platform.SandboxDir(v)
		sb, err := parseSandbox(runtime, dir)
		if err != nil {
			a.logger.Warn("cannot read sandbox", slog.String("dir", dir))
			continue
		}

		if selected[sb.Spec().Name] {
			result = append(result, sb)
		}
	}
	fmt.Println(result)

	a.logger.Debug("List done", slog.String("dir", a.dir))
	return result, nil
}

func (a *api) Create(ctx context.Context, spec string) (Sandbox, error) {
	a.logger.Debug("Create start", slog.String("dir", a.dir))

	ss, ok := a.registry.SandboxSpec(spec)
	if !ok {
		return nil, fmt.Errorf("%w: sandbox spec %q", ErrNotFound, spec)
	}

	// TODO: validate max_instances
	sandbox, err := newSandbox(ctx, a.makeRuntime(), ss)
	if err != nil {
		return nil, err
	}

	a.logger.Debug("Create end", slog.String("dir", a.dir))
	return sandbox, nil
}

var _ API = (*api)(nil)
