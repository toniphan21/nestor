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

	Build() error
}

const DefaultLogFile = "nestor.log"

func New(options ...Option) (API, error) {
	o := &opts{
		logger:   slog.New(slog.DiscardHandler),
		registry: newRegistry(),
	}

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
		platform: o.platform.Clone(o.dir),
		logger:   o.logger,
		registry: o.registry,
	}

	// register sandboxSpecs and harnessSpecs passed via options
	if len(o.harnessSpecs) > 0 {
		for _, v := range o.harnessSpecs {
			a.registry.RegisterHardnessSpec(v)
		}
	}

	return a, nil
}

type api struct {
	dir      string
	platform Platform
	registry Registry
	logger   *slog.Logger
	inited   bool
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

	// sandbox.yml is saved from assets for the first time in NESTOR_DIR/harness.yml
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

	// harness.yml is saved from assets for the first time in NESTOR_DIR/harness.yml
	hf := a.platform.HarnessYmlFile()
	if !fs.HasFile(hf) {
		if err := fs.CopyFileFS(builtin, "assets/harness.yml", hf); err != nil {
			a.logger.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.logger.Info("saved builtin harness.yml file", slog.String("path", hf))
	}

	// if there is no harness specs provided, parse from NESTOR_DIR/harness.yml
	// for other location the caller need to parse manually add pass via WithHarnessSpecs
	if len(a.registry.HarnessSpecs()) == 0 {
		yml, err := fs.AtomicReadFile(hf)
		if err != nil {
			e := fmt.Errorf("cannot read harness.yml file: %w", err)
			a.logger.Error(e.Error(), slog.Any("error", e), slog.String("path", hf))
			return e
		}
		hss, err := ParseHarnessSpecs(bytes.NewBuffer(yml))
		if err != nil {
			e := fmt.Errorf("cannot parse harness.yml file: %w", err)
			a.logger.Error(e.Error(), slog.Any("error", e), slog.String("path", hf))
			return e
		}

		for _, v := range hss {
			a.registry.RegisterHardnessSpec(v)
		}
		a.logger.Info("loaded harness specs", slog.String("path", hf))
	} else {
		a.logger.Info("use harness specs from WithHarnessSpecs")
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

func (a *api) Build() error {
	if !a.inited {
		return ErrInitRequired
	}
	a.logger.Info("Build start", slog.String("dir", a.dir))

	ctx := a.makeContext(context.Background())
	for _, sandbox := range a.registry.SandboxSpecs() {
		hn, have := a.registry.Harness(string(sandbox.Harness))
		if !have {
			return fmt.Errorf("%w: harness %q", ErrNotFound, sandbox.Harness)
		}
		hns, err := hn.Spec(ctx)
		if err != nil {
			return err
		}

		fmt.Println("path:", filepath.Dir(hns.Dockerfile))
		fmt.Println("dockerfile:", hns.Dockerfile)
		fmt.Println("target:", sandbox.Target)
		fmt.Println("tag:", fmt.Sprintf("nestor-%s", sandbox.Name))

		id, err := docker.Build(ctx, filepath.Dir(hns.Dockerfile), docker.BuildOptions{
			Dockerfile: hns.Dockerfile,
			Target:     sandbox.Target,
			Tag:        fmt.Sprintf("nestor-%s", sandbox.Name),
		}, a.logger)
		if err != nil {
			return err
		}
		fmt.Println("id:", id)
	}

	a.logger.Info("Build done", slog.String("dir", a.dir))
	return nil
}

var _ API = (*api)(nil)
