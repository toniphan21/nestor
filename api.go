package nestor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"nhatp.com/go/nestor/infra/fs"
)

type API interface {
	Runtime() Runtime

	Build(ctx context.Context, specs ...string) error

	ListSandboxes(ctx context.Context, specs ...string) ([]Sandbox, error)

	CreateSandbox(ctx context.Context, spec string) (Sandbox, error)

	Acquire(ctx context.Context, spec string, path string) (*Lease, error)
}

const DefaultLogFile = "nestor.log"
const DefaultHostAlias = "nestor-proxy.internal"

func DefaultLogger(level slog.Level, options ...Option) (*slog.Logger, io.Closer, error) {
	o := &opts{
		template: DefaultTemplate(),
	}
	for _, v := range options {
		v.apply(o)
	}

	if o.platform == nil {
		defaultPlatform, err := DefaultPlatform()
		if err != nil {
			return nil, nil, err
		}
		o.platform = defaultPlatform
	}

	if strings.TrimSpace(o.dir) == "" {
		o.dir = o.platform.NestorDir()
	}
	fp := filepath.Join(o.dir, DefaultLogFile)

	return NewLogger(fp, io.Discard, level)
}

func New(options ...Option) (API, error) {
	o := &opts{
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

	if strings.TrimSpace(o.dir) == "" {
		o.dir = o.platform.NestorDir()
	}

	if o.logger == nil {
		o.logger = slog.New(slog.DiscardHandler)
	}

	if o.newGitFunc == nil {
		o.newGitFunc = newGitCLI
	}

	if o.newDockerFunc == nil {
		o.newDockerFunc = newDockerCLI
	}

	if err := fs.MkdirAll(o.dir); err != nil {
		return nil, err
	}

	a := &api{
		dir:           o.dir,
		platform:      o.platform.Clone(o.dir),
		registry:      o.registry,
		template:      o.template,
		newGitFunc:    o.newGitFunc,
		newDockerFunc: o.newDockerFunc,
		logger:        o.logger.With(slog.String("lib", "nestor")),
		log:           o.logger.With(slog.String("lib", "nestor"), slog.String("layer", "api")),
	}
	a.docker = a.newDockerFunc(a.log)

	// register profiles, sandboxSpecs and harnesses passed via options
	for _, v := range o.profiles {
		a.registry.RegisterProfile(v)
	}
	for _, v := range o.sandboxSpecs {
		a.registry.RegisterSandboxSpec(v)
	}
	for _, v := range o.harnesses {
		a.registry.RegisterHarness(v)
	}

	if err := a.init(); err != nil {
		return nil, err
	}
	return a, nil
}

type api struct {
	mu            sync.Mutex
	dir           string
	platform      Platform
	registry      Registry
	template      Template
	newGitFunc    NewGitFunc
	newDockerFunc NewDockerFunc
	docker        Docker
	logger        *slog.Logger
	log           *slog.Logger
}

func (a *api) init() error {
	a.log.Debug("Init start", slog.String("dir", a.dir))
	runtime := a.Runtime()

	// profile.yml is saved from assets for the first time in NESTOR_DIR/profile.yml
	pf := a.platform.ProfileYmlFile()
	if !fs.HasFile(pf) {
		if err := fs.CopyFileFS(Embed, "assets/profile.yml", pf); err != nil {
			a.log.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.log.Info("saved builtin profile.yml file", slog.String("path", pf))
	}

	// if there is no profiles provided, parse from NESTOR_DIR/profile.yml
	// for other location the caller need to parse manually add pass via WithProfiles
	if len(a.registry.Profiles()) == 0 {
		yml, err := fs.AtomicReadFile(pf)
		if err != nil {
			e := fmt.Errorf("cannot read profile.yml file: %w", err)
			a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", pf))
			return e
		}
		hss, err := ParseProfiles(bytes.NewBuffer(yml))
		if err != nil {
			e := fmt.Errorf("cannot parse profile.yml file: %w", err)
			a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", pf))
			return e
		}

		for _, v := range hss {
			a.registry.RegisterProfile(v)
		}
		a.log.Info("loaded harness specs", slog.String("path", pf))
	} else {
		a.log.Info("use harness specs from WithProfiles")
	}

	// initialize builtin harnesses
	for _, v := range a.registry.Harnesses() {
		if err := v.Init(runtime); err != nil {
			return err
		}
	}
	a.log.Debug("builtin harnesses initialized")

	// sandbox.yml is saved from assets for the first time in NESTOR_DIR/sandbox.yml
	sf := a.platform.SandboxYmlFile()
	if !fs.HasFile(sf) {
		if err := fs.CopyFileFS(Embed, "assets/sandbox.yml", sf); err != nil {
			a.log.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.log.Info("saved builtin sandbox.yml file", slog.String("path", sf))
	}

	// if there is no sandbox provided, parse from NESTOR_DIR/sandbox.yml
	// for other location the caller need to parse manually add pass via WithSandboxSpecs
	if len(a.registry.SandboxSpecs()) == 0 {
		yml, err := fs.AtomicReadFile(sf)
		if err != nil {
			e := fmt.Errorf("cannot read sandbox.yml file: %w", err)
			a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", sf))
			return e
		}
		sbs, err := ParseSandboxSpecs(runtime, bytes.NewBuffer(yml))
		if err != nil {
			e := fmt.Errorf("cannot parse sandbox.yml file: %w", err)
			a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", sf))
			return e
		}

		for _, v := range sbs {
			a.registry.RegisterSandboxSpec(v)
		}
		a.log.Info("loaded sandbox specs", slog.String("path", sf))
	} else {
		a.log.Info("use sandbox specs from WithSandboxSpecs")
	}

	a.log.Debug("Init done", slog.String("dir", a.dir))
	return nil
}

func (a *api) Runtime() Runtime {
	return Runtime{
		Registry:      a.registry,
		Platform:      a.platform,
		Template:      a.template,
		Logger:        a.logger,
		newGitFunc:    a.newGitFunc,
		newDockerFunc: a.newDockerFunc,
	}
}

func (a *api) Build(ctx context.Context, specs ...string) error {
	a.log.Debug("Build start", slog.String("dir", a.dir))
	runtime := a.Runtime()

	var selected = make(map[string]bool)
	if len(specs) == 0 {
		for _, v := range a.registry.SandboxSpecs() {
			selected[v.Name] = true
		}
	} else {
		for _, v := range specs {
			selected[v] = true
		}
	}

	for _, spec := range a.registry.SandboxSpecs() {
		if !selected[spec.Name] {
			continue
		}

		h, p, err := spec.findHarnessAndProfile(runtime)
		if err != nil {
			return err
		}

		dockerfile := p.dockerfile(h, runtime)
		buildPath := filepath.Dir(dockerfile)
		options := DockerBuildOption{
			Dockerfile: dockerfile,
			Target:     spec.Target,
			Tag:        a.template.MakeSandboxTag(spec.Name),
		}

		if _, err = a.docker.Build(ctx, buildPath, options); err != nil {
			return err
		}
	}

	a.log.Debug("Build done", slog.String("dir", a.dir))
	return nil
}

func (a *api) ListSandboxes(ctx context.Context, specs ...string) ([]Sandbox, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Debug("ListSandboxes start", slog.String("dir", a.dir))

	result, err := a.listSandboxesLocked(ctx, specs...)

	a.log.Debug("ListSandboxes done", slog.String("dir", a.dir))
	return result, err
}

func (a *api) listSandboxesLocked(ctx context.Context, specs ...string) ([]Sandbox, error) {
	if !fs.HasDir(a.platform.SandboxDir()) {
		return nil, nil
	}

	var selected = make(map[string]bool)
	if len(specs) == 0 {
		for _, v := range a.registry.SandboxSpecs() {
			selected[v.Name] = true
		}
	} else {
		for _, v := range specs {
			selected[v] = true
		}
	}

	var result []Sandbox
	dirs, err := fs.ListDirs(a.platform.SandboxDir())
	if err != nil {
		return nil, err
	}

	runtime := a.Runtime()
	for _, v := range dirs {
		dir := a.platform.SandboxDir(v)
		sb, err := parseSandbox(runtime, dir)
		if err != nil {
			// when sandbox spec not found, it maybe deleted - nothing to warn
			if errors.Is(err, ErrNotFound) {
				continue
			}

			a.log.Warn("cannot read sandbox", slog.String("dir", dir))
			continue
		}

		if selected[sb.Spec().Name] {
			result = append(result, sb)
		}
	}
	return result, nil
}

func (a *api) CreateSandbox(ctx context.Context, spec string) (Sandbox, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Debug("CreateSandbox start", slog.String("dir", a.dir))

	sandbox, err := a.createSandboxLocked(ctx, spec)

	a.log.Debug("CreateSandbox end", slog.String("dir", a.dir))
	return sandbox, err
}

func (a *api) createSandboxLocked(ctx context.Context, spec string) (Sandbox, error) {
	ss, ok := a.registry.SandboxSpec(spec)
	if !ok {
		return nil, fmt.Errorf("%w: sandbox spec %q", ErrNotFound, spec)
	}

	available, err := a.listSandboxesLocked(ctx, spec)
	if err != nil {
		return nil, err
	}
	if len(available) == ss.MaxInstances {
		return nil, fmt.Errorf("%w: exceed maximum instances", ErrNotAllowed)
	}

	sandbox, err := newSandbox(ctx, a.Runtime(), ss)
	if err != nil {
		return nil, err
	}
	return sandbox, nil
}

func (a *api) Acquire(ctx context.Context, spec string, path string) (*Lease, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log.Debug("Acquire start", slog.String("dir", a.dir))

	ss, ok := a.registry.SandboxSpec(spec)
	if !ok {
		return nil, fmt.Errorf("%w: sandbox spec %q", ErrNotFound, spec)
	}

	if !a.docker.HasImage(ctx, a.template.MakeSandboxTag(ss.Name)) {
		a.log.Debug("no image, build fresh one", slog.String("dir", a.dir))
		if err := a.Build(ctx, spec); err != nil {
			return nil, err
		}
	}

	// search exists first
	sandboxes, err := a.listSandboxesLocked(ctx, spec)
	if err != nil {
		return nil, err
	}

	for _, sandbox := range sandboxes {
		lease, err := sandbox.Acquire(ctx, path)
		if err == nil {
			return lease, nil
		}

		if !errors.Is(err, ErrNotAvailable) {
			return nil, err
		}
	}

	// create new one
	sandbox, err := a.createSandboxLocked(ctx, spec)
	if err != nil {
		if errors.Is(err, ErrNotAllowed) {
			return nil, fmt.Errorf("%w: %w", ErrNotAvailable, err)
		}
		return nil, err
	}
	lease, err := sandbox.Acquire(ctx, path)
	if err != nil {
		return nil, err
	}

	a.log.Debug("Acquire end", slog.String("dir", a.dir))
	return lease, nil
}

var _ API = (*api)(nil)
