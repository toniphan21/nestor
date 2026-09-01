package nestor

import (
	"context"
	"log/slog"

	"nhatp.com/go/nestor/infra/git"
)

type GitWorktree struct {
	Path   string `json:"path"`
	Head   string `json:"head"`
	Branch string `json:"branch"`
}

type NewGitFunc func(*slog.Logger) Git

type Git interface {
	RemoveBranchForce(ctx context.Context, repository, branch string) error

	ListWorktrees(ctx context.Context, repository string) ([]GitWorktree, error)

	AddWorktree(ctx context.Context, repository, dir, initialBranch string) error

	RemoveWorktree(ctx context.Context, repository, dir, initialBranch string) error
}

func newGitCLI(logger *slog.Logger) Git {
	return &gitCLI{
		log: logger.With("component", "gitCLI"),
	}
}

type gitCLI struct {
	log *slog.Logger
}

func (g *gitCLI) RemoveBranchForce(ctx context.Context, repository, branch string) error {
	return git.RemoveBranchForce(ctx, repository, branch, g.log)
}

func (g *gitCLI) ListWorktrees(ctx context.Context, repository string) ([]GitWorktree, error) {
	out, err := git.ListWorktrees(ctx, repository, g.log)
	if err != nil {
		return nil, err
	}

	var result []GitWorktree
	for _, v := range out {
		result = append(result, GitWorktree{
			Path:   v.Path,
			Head:   v.Head,
			Branch: v.Branch,
		})
	}
	return result, nil
}

func (g *gitCLI) AddWorktree(ctx context.Context, repository, dir, initialBranch string) error {
	return git.AddWorktree(ctx, repository, dir, initialBranch, g.log)
}

func (g *gitCLI) RemoveWorktree(ctx context.Context, repository, dir, initialBranch string) error {
	return git.RemoveWorktree(ctx, repository, dir, initialBranch, g.log)
}

var _ Git = (*gitCLI)(nil)
