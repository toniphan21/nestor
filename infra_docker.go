package nestor

import (
	"context"
	"log/slog"

	"nhatp.com/go/nestor/infra/docker"
)

type DockerBuildOptions struct {
	Target     string
	Tag        string
	Dockerfile string
}

type NewDockerFunc func(*slog.Logger) Docker

type Docker interface {
	Build(ctx context.Context, path string, options DockerBuildOptions) (string, error)

	IsRunning(ctx context.Context, container string) bool

	Kill(ctx context.Context, container string) error
}

func newDockerCLI(logger *slog.Logger) Docker {
	return &dockerCLI{
		log: logger.With("component", "dockerCLI"),
	}
}

type dockerCLI struct {
	log *slog.Logger
}

func (d *dockerCLI) Build(ctx context.Context, path string, options DockerBuildOptions) (string, error) {
	o := docker.BuildOptions{
		Target:     options.Target,
		Tag:        options.Tag,
		Dockerfile: options.Dockerfile,
	}
	return docker.Build(ctx, path, o, d.log)
}

func (d *dockerCLI) IsRunning(ctx context.Context, container string) bool {
	return docker.IsRunning(ctx, container, d.log)
}

func (d *dockerCLI) Kill(ctx context.Context, container string) error {
	return docker.Kill(ctx, container, d.log)
}

var _ Docker = (*dockerCLI)(nil)
