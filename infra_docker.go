package nestor

import (
	"context"
	"log/slog"
	"time"

	"nhatp.com/go/nestor/infra/docker"
)

type DockerBuildOption struct {
	Target     string
	Tag        string
	Dockerfile string
}

type DockerRunOption struct {
	Mounts []string
}

type NewDockerFunc func(*slog.Logger) Docker

type Docker interface {
	Build(ctx context.Context, path string, opt DockerBuildOption) (string, error)

	HasImage(ctx context.Context, ref string) bool

	IsRunning(ctx context.Context, container string) bool

	Kill(ctx context.Context, container string) error

	Stop(ctx context.Context, container string, timeout time.Duration) error

	Run(ctx context.Context, image, name string, opt DockerRunOption) (string, error)
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
	o := docker.BuildOptions{
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
	o := docker.RunOptions{
		Mounts: opt.Mounts,
	}
	return docker.Run(ctx, image, name, o, d.log)
}

var _ Docker = (*dockerCLI)(nil)
