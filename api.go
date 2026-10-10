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

	RenameSandbox(ctx context.Context, sandbox Sandbox, newName string) error

	Acquire(ctx context.Context, spec string, path string) (*Lease, error)
}

const (
	DefaultLogFile   = "nestor.log"
	DefaultHostAlias = "nestor-proxy.internal"
)

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

	if strings.TrimSpace(o.configDir) == "" {
		o.configDir = o.platform.ConfigDir()
	}
	if strings.TrimSpace(o.dataDir) == "" {
		o.dataDir = o.platform.DataDir()
	}
	if strings.TrimSpace(o.stateDir) == "" {
		o.stateDir = o.platform.StateDir()
	}

	if err := fs.MkdirAll(o.stateDir); err != nil {
		return nil, nil, err
	}
	fp := filepath.Join(o.stateDir, DefaultLogFile)

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

	if strings.TrimSpace(o.configDir) == "" {
		o.configDir = o.platform.ConfigDir()
	}
	if strings.TrimSpace(o.dataDir) == "" {
		o.dataDir = o.platform.DataDir()
	}
	if strings.TrimSpace(o.stateDir) == "" {
		o.stateDir = o.platform.StateDir()
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

	if err := fs.MkdirAll(o.configDir); err != nil {
		return nil, err
	}
	if err := fs.MkdirAll(o.dataDir); err != nil {
		return nil, err
	}

	a := &api{
		configDir:     o.configDir,
		dataDir:       o.dataDir,
		stateDir:      o.stateDir,
		platform:      o.platform.Clone(o.configDir, o.dataDir, o.stateDir),
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
	for _, v := range o.mcps {
		a.registry.RegisterMCP(v)
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
	configDir     string
	dataDir       string
	stateDir      string
	platform      Platform
	registry      Registry
	template      Template
	newGitFunc    NewGitFunc
	newDockerFunc NewDockerFunc
	docker        Docker
	logger        *slog.Logger
	log           *slog.Logger
}

func (a *api) logDebug(msg string, args ...any) {
	p := append([]any{
		slog.String("configDir", a.configDir),
		slog.String("dataDir", a.dataDir),
		slog.String("stateDir", a.stateDir),
	}, args...)

	a.log.Debug(msg, p...)
}

func (a *api) init() error {
	a.logDebug("Init start")
	steps := []func() error{
		a.initProfilesFromYmlIfRuntimeHasNoProfiles,
		a.initMCPFromYmlIfRuntimeHasNoMCPs,
		a.initBuiltinHarnesses,
		a.initSpecsFromYmlIfRuntimeHasNoSpecs,
		a.initSandboxSlugs,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	a.logDebug("Init done")
	return nil
}

func (a *api) initProfilesFromYmlIfRuntimeHasNoProfiles() error {
	if len(a.registry.Profiles()) != 0 {
		a.log.Info("use profile from WithProfiles")
		return nil
	}

	// profile.yml is saved from assets for the first time in NESTOR_DIR/profile.yml
	fp := a.platform.ProfileYmlFile()
	if !fs.HasFile(fp) {
		if err := fs.CopyFileFS(Embed, "assets/profile.yml", fp); err != nil {
			a.log.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.log.Info("saved builtin profile.yml file", slog.String("path", fp))
	}

	yml, err := fs.AtomicReadFile(fp)
	if err != nil {
		e := fmt.Errorf("cannot read profile.yml file: %w", err)
		a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", fp))
		return e
	}
	hss, err := ParseProfiles(bytes.NewBuffer(yml))
	if err != nil {
		e := fmt.Errorf("cannot parse profile.yml file: %w", err)
		a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", fp))
		return e
	}

	for _, v := range hss {
		a.registry.RegisterProfile(v)
	}
	a.log.Info("loaded profile from profile.yml", slog.String("path", fp))
	return nil
}

func (a *api) initBuiltinHarnesses() error {
	// initialize builtin harnesses
	for _, v := range a.registry.Harnesses() {
		if err := v.Init(a.Runtime()); err != nil {
			return err
		}
	}
	a.logDebug("builtin harnesses initialized")
	return nil
}

func (a *api) initMCPFromYmlIfRuntimeHasNoMCPs() error {
	if len(a.registry.MCPs()) != 0 {
		a.log.Info("use MCPs from WithMCPs")
		return nil
	}

	// mcp.yml is saved from assets for the first time in NESTOR_DIR/mcp.yml
	fp := a.platform.MCPYmlFile()
	if !fs.HasFile(fp) {
		if err := fs.CopyFileFS(Embed, "assets/mcp.yml", fp); err != nil {
			a.log.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.log.Info("saved builtin mcp.yml file", slog.String("path", fp))
	}

	yml, err := fs.AtomicReadFile(fp)
	if err != nil {
		e := fmt.Errorf("cannot read mcp.yml file: %w", err)
		a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", fp))
		return e
	}
	mcps, err := ParseMCPs(bytes.NewBuffer(yml))
	if err != nil {
		e := fmt.Errorf("cannot parse mcp.yml file: %w", err)
		a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", fp))
		return e
	}

	for _, v := range mcps {
		a.registry.RegisterMCP(v)
	}
	a.log.Info("loaded MCPs from mcp.yml", slog.String("path", fp))
	return nil
}

func (a *api) initSpecsFromYmlIfRuntimeHasNoSpecs() error {
	if len(a.registry.SandboxSpecs()) != 0 {
		a.log.Info("use sandbox specs from WithSandboxSpecs")
		return nil
	}

	// sandbox.yml is saved from assets for the first time in NESTOR_DIR/sandbox.yml
	fp := a.platform.SandboxYmlFile()
	if !fs.HasFile(fp) {
		if err := fs.CopyFileFS(Embed, "assets/sandbox.yml", fp); err != nil {
			a.log.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.log.Info("saved builtin sandbox.yml file", slog.String("path", fp))
	}

	yml, err := fs.AtomicReadFile(fp)
	if err != nil {
		e := fmt.Errorf("cannot read sandbox.yml file: %w", err)
		a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", fp))
		return e
	}
	sbs, err := ParseSandboxSpecs(a.Runtime(), bytes.NewBuffer(yml))
	if err != nil {
		e := fmt.Errorf("cannot parse sandbox.yml file: %w", err)
		a.log.Error(e.Error(), slog.Any("error", e), slog.String("path", fp))
		return e
	}

	for _, v := range sbs {
		a.registry.RegisterSandboxSpec(v)
	}
	a.log.Info("loaded sandbox specs from sandbox.yml", slog.String("path", fp))
	return nil
}

func (a *api) initSandboxSlugs() error {
	ssf := a.platform.SandboxSlugsFile()
	if !fs.HasFile(ssf) {
		if err := fs.CopyFileFS(Embed, "assets/sandbox-slugs.txt", ssf); err != nil {
			a.log.Error(err.Error(), slog.Any("error", err))
			return err
		}
		a.log.Info("saved builtin sandbox-slugs.txt file", slog.String("path", ssf))
	}

	b, err := fs.AtomicReadFile(ssf)
	if err != nil {
		return err
	}
	_ = a.template.LoadSandboxSlugs(bytes.NewBuffer(b))
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
	a.logDebug("Build start")
	runtime := a.Runtime()

	selected := make(map[string]bool)
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

		if err := spec.Build(ctx, runtime, a.docker, DockerBuildOption{}); err != nil {
			return err
		}
	}

	a.logDebug("Build done")
	return nil
}

func (a *api) ListSandboxes(ctx context.Context, specs ...string) ([]Sandbox, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.listSandboxesLocked(ctx, specs...)
}

func (a *api) listSandboxesLocked(ctx context.Context, specs ...string) ([]Sandbox, error) {
	if !fs.HasDir(a.platform.SandboxDir()) {
		return nil, nil
	}

	selected := make(map[string]bool)
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

	a.logDebug("CreateSandbox start")

	sandbox, err := a.createSandboxLocked(ctx, spec)

	a.logDebug("CreateSandbox end")
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

func (a *api) RenameSandbox(ctx context.Context, sandbox Sandbox, newName string) error {
	s, ok := sandbox.(*sandboxImpl)
	if !ok {
		return errUnknownSandbox
	}
	return s.rename(ctx, newName)
}

func (a *api) Acquire(ctx context.Context, spec string, path string) (*Lease, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logDebug("Acquire start", slog.String("spec", spec), slog.String("path", path))

	ss, ok := a.registry.SandboxSpec(spec)
	if !ok {
		return nil, fmt.Errorf("%w: sandbox spec %q", ErrNotFound, spec)
	}

	lease, err := a.acquireLocked(ctx, spec, path)
	if err != nil {
		return nil, err
	}

	if !a.docker.HasImage(ctx, ss.DockerImageName(a.template)) {
		a.logDebug("no image, build fresh one")
		if err := a.Build(ctx, spec); err != nil {
			return nil, err
		}
	}
	a.logDebug("Acquire end")
	return lease, nil
}

func (a *api) acquireLocked(ctx context.Context, spec string, path string) (*Lease, error) {
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
	return lease, nil
}

var _ API = (*api)(nil)
