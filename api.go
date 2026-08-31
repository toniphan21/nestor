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
	Init() error

	Build(sandboxes ...string) error

	Sync(sandbox string) error

	Start(sandbox string) error
}

const DefaultLogFile = "nestor.log"
const DefaultSandboxTagTemplate = "nestor-[sandboxName]"

func New(options ...Option) (API, error) {
	o := &opts{
		logger:             slog.New(slog.DiscardHandler),
		registry:           newRegistry(),
		sandboxTagTemplate: DefaultSandboxTagTemplate,
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
	}

	// register sandboxSpecs and profiles passed via options
	if len(o.profiles) > 0 {
		for _, v := range o.profiles {
			a.registry.RegisterProfile(v)
		}
	}

	return a, nil
}

type api struct {
	dir                string
	platform           Platform
	registry           Registry
	logger             *slog.Logger
	inited             bool
	sandboxTagTemplate string
}

func (a *api) makeContext(ctx context.Context) Context {
	return newContext(ctx, a.platform, a.registry)
}

func (a *api) Init() error {
	a.logger.Info("Init start", slog.String("dir", a.dir))
	if a.inited {
		a.logger.Info("Init already, done", slog.String("dir", a.dir))
		return nil
	}
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
	a.inited = true
	return nil
}

func (a *api) Build(sandboxes ...string) error {
	if !a.inited {
		return ErrInitRequired
	}
	a.logger.Info("Build start", slog.String("dir", a.dir))

	ctx := a.makeContext(context.Background())

	var selected = make(map[string]bool)
	if len(sandboxes) == 0 {
		for _, v := range a.registry.SandboxSpecs() {
			selected[v.Name] = true
		}
	}

	for _, sandbox := range a.registry.SandboxSpecs() {
		if !selected[sandbox.Name] {
			continue
		}

		r, err := sandbox.ResolveHarness(ctx)
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

func (a *api) Sync(sandbox string) error {
	//TODO implement me
	panic("implement me")
}

func (a *api) Start(sandbox string) error {
	//TODO implement me
	panic("implement me")
}

var _ API = (*api)(nil)
