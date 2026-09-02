package nestor

import (
	"context"
	"io"
	"log/slog"
	"time"

	"nhatp.com/go/nestor/infra/docker"
)

type DockerBuildOption struct {
	Target     string
	Tag        string
	Dockerfile string
}

type DockerMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type DockerRunOption struct {
	Mounts []DockerMount
}

type DockerExecOption struct {
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
}

type NewDockerFunc func(*slog.Logger) Docker

type Docker interface {
	Build(ctx context.Context, path string, opt DockerBuildOption) (string, error)

	HasImage(ctx context.Context, ref string) bool

	IsRunning(ctx context.Context, container string) bool

	Kill(ctx context.Context, container string) error

	Stop(ctx context.Context, container string, timeout time.Duration) error

	Run(ctx context.Context, image, name string, opt DockerRunOption) (string, error)

	Exec(ctx context.Context, container string, commands []string, opt DockerExecOption) (int, error)
}

func newDockerCLI(logger *slog.Logger) Docker {
	return &dockerCLI{
		log: logger.With("component", "dockerCLI"),
	}
}

type dockerCLI struct {
	log *slog.Logger
}

func (d *dockerCLI) Build(ctx context.Context, path string, opt DockerBuildOption) (string, error) {
	o := docker.BuildOption{
		Target:     opt.Target,
		Tag:        opt.Tag,
		Dockerfile: opt.Dockerfile,
	}
	return docker.Build(ctx, path, o, d.log)
}

func (d *dockerCLI) HasImage(ctx context.Context, ref string) bool {
	return docker.HasImage(ctx, ref, d.log)
}

func (d *dockerCLI) IsRunning(ctx context.Context, container string) bool {
	return docker.IsRunning(ctx, container, d.log)
}

func (d *dockerCLI) Kill(ctx context.Context, container string) error {
	return docker.Kill(ctx, container, d.log)
}

func (d *dockerCLI) Stop(ctx context.Context, container string, timeout time.Duration) error {
	return docker.Stop(ctx, container, timeout, d.log)
}

func (d *dockerCLI) Run(ctx context.Context, image, name string, opt DockerRunOption) (string, error) {
	var mounts []docker.Mount
	for _, v := range opt.Mounts {
		mounts = append(mounts, docker.Mount{Source: v.Source, Target: v.Target, ReadOnly: v.ReadOnly})
	}
	o := docker.RunOption{
		Mounts: mounts,
	}
	return docker.Run(ctx, image, name, o, d.log)
}

func (d *dockerCLI) Exec(ctx context.Context, container string, commands []string, opt DockerExecOption) (int, error) {
	options := docker.ExecOption{
		WorkDir: opt.WorkDir,
		Stdout:  opt.Stdout,
		Stderr:  opt.Stderr,
	}
	return docker.Exec(ctx, container, commands, options, d.log)
}

var _ Docker = (*dockerCLI)(nil)
