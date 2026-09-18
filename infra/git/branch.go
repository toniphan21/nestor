package git

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

type logWriter struct {
	log   *slog.Logger
	level slog.Level
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.log.Log(context.Background(), w.level, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func RemoveBranchForce(ctx context.Context, repository, branch string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "branch", "-D", branch,
	}
	cmd := exec.CommandContext(ctx, "git", args...)

	log := logger.WithGroup("git").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	log.Info("git branch -D", slog.String("repository", repository), slog.String("branch", branch))
	if err := cmd.Run(); err != nil {
		log.Error("git branch -D", slog.Any("error", err))
		return fmt.Errorf("git branch -D: %w", err)
	}
	return nil
}

func HasBranch(ctx context.Context, repository, branch string, logger *slog.Logger) (bool, error) {
	args := []string{
		"-C", repository, "show-ref", "--verify", "--quiet", "refs/heads/" + branch,
	}
	cmd := exec.CommandContext(ctx, "git", args...)

	log := logger.WithGroup("git").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	log.Info("git show-ref", slog.String("repository", repository), slog.String("branch", "refs/heads/"+branch))
	if err := cmd.Run(); err != nil {
		log.Error("git branch -D", slog.Any("error", err))
		return false, fmt.Errorf("git show-ref: %w", err)
	}
	return true, nil
}

func RenameBranch(ctx context.Context, repository, oldBranch, newBranch string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "branch", "-m", oldBranch, newBranch,
	}
	cmd := exec.CommandContext(ctx, "git", args...)

	log := logger.WithGroup("git").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	log.Info("git branch -m", slog.String("repository", repository), slog.String("oldBranch", oldBranch), slog.String("newBranch", newBranch))
	if err := cmd.Run(); err != nil {
		log.Error("git branch -D", slog.Any("error", err))
		return fmt.Errorf("git branch -m: %w", err)
	}
	return nil
}
