package nestor

import (
	"context"
	"log/slog"
	"time"
)

func NoopGit(logger *slog.Logger) Git {
	return &noopGit{
		log: logger.With("component", "noopGit"),
	}
}

type noopGit struct {
	log *slog.Logger
}

func (n *noopGit) RemoveBranchForce(ctx context.Context, repository, branch string) error {
	n.log.Debug("RemoveBranchForce", slog.String("repository", repository), slog.String("branch", branch))
	return nil
}

func (n *noopGit) ListWorktrees(ctx context.Context, repository string) ([]GitWorktree, error) {
	n.log.Debug("ListWorktrees", slog.String("repository", repository))
	return nil, nil
}

func (n *noopGit) AddWorktree(ctx context.Context, repository, dir, initialBranch string) error {
	n.log.Debug("AddWorktree", slog.String("repository", repository), slog.String("dir", dir), slog.String("initialBranch", initialBranch))
	return nil
}

func (n *noopGit) RemoveWorktree(ctx context.Context, repository, dir, initialBranch string) error {
	n.log.Debug("RemoveWorktree", slog.String("repository", repository), slog.String("dir", dir), slog.String("initialBranch", initialBranch))
	return nil
}

var _ Git = (*noopGit)(nil)

func NoopDocker(logger *slog.Logger) Docker {
	return &noopDocker{
		log: logger.With("component", "noopDocker"),
	}
}

type noopDocker struct {
	log *slog.Logger
}

func (n *noopDocker) Build(ctx context.Context, path string, opt DockerBuildOption) (string, error) {
	n.log.Debug("RemoveWorktree", slog.String("path", path), slog.Any("options", opt))
	return "", nil
}

func (n *noopDocker) HasImage(ctx context.Context, ref string) bool {
	n.log.Debug("HasImage", slog.String("ref", ref))
	return false
}

func (n *noopDocker) IsRunning(ctx context.Context, container string) bool {
	n.log.Debug("IsRunning", slog.String("container", container))
	return false
}

func (n *noopDocker) Kill(ctx context.Context, container string) error {
	n.log.Debug("Kill", slog.String("container", container))
	return nil
}

func (n *noopDocker) Stop(ctx context.Context, container string, timeout time.Duration) error {
	n.log.Debug("Stop", slog.String("container", container))
	return nil
}

func (n *noopDocker) Run(ctx context.Context, image, name string, opt DockerRunOption) (string, error) {
	n.log.Debug("Run", slog.String("image", image), slog.String("name", name), slog.Any("opt", opt))
	return "", nil
}

var _ Docker = (*noopDocker)(nil)
