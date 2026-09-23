package nestor

import (
	"log/slog"
)

type Option interface {
	apply(*opts)
}

type optionFunc func(*opts)

func (f optionFunc) apply(opts *opts) { f(opts) }

type opts struct {
	dir           string
	logger        *slog.Logger
	platform      Platform
	registry      Registry
	template      Template
	sandboxSpecs  []SandboxSpec
	profiles      []Profile
	mcps          []MCP
	harnesses     []Harness
	newGitFunc    NewGitFunc
	newDockerFunc NewDockerFunc
}

func WithDir(dir string) Option {
	return optionFunc(func(opts *opts) { opts.dir = dir })
}

func WithLogger(logger *slog.Logger) Option {
	return optionFunc(func(o *opts) { o.logger = logger })
}

func WithPlatform(platform Platform) Option {
	return optionFunc(func(o *opts) { o.platform = platform })
}

func WithSandboxSpecs(specs []SandboxSpec) Option {
	return optionFunc(func(o *opts) { o.sandboxSpecs = specs })
}

func WithProfiles(profiles []Profile) Option {
	return optionFunc(func(o *opts) { o.profiles = profiles })
}

func WithMCPs(mcps []MCP) Option {
	return optionFunc(func(o *opts) { o.mcps = mcps })
}

func WithHarnesses(harnesses Harness) Option {
	return optionFunc(func(o *opts) { o.harnesses = o.harnesses })
}

func WithTemplate(template Template) Option {
	return optionFunc(func(opts *opts) { opts.template = template })
}

func WithGit(fn NewGitFunc) Option {
	return optionFunc(func(opts *opts) { opts.newGitFunc = fn })
}

func WithDocker(fn NewDockerFunc) Option {
	return optionFunc(func(opts *opts) { opts.newDockerFunc = fn })
}
