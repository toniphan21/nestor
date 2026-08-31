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
	dir          string
	logger       *slog.Logger
	platform     Platform
	registry     Registry
	sandboxSpecs []SandboxSpec
	harnessSpecs []HarnessSpec
}

func WithDir(dir string) Option {
	return optionFunc(func(opts *opts) { opts.dir = dir })
}

func WithLogger(logger *slog.Logger) Option {
	return optionFunc(func(opts *opts) { opts.logger = logger })
}

func WithPlatform(platform Platform) Option {
	return optionFunc(func(opts *opts) {
		opts.platform = platform
	})
}

func WithSandboxSpecs(specs []SandboxSpec) Option {
	return optionFunc(func(opts *opts) {
		opts.sandboxSpecs = specs
	})
}

func WithHarnessSpecs(specs []HarnessSpec) Option {
	return optionFunc(func(opts *opts) {
		opts.harnessSpecs = specs
	})
}
