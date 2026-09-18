package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

type Worktree struct {
	Path   string `json:"path"`
	Head   string `json:"head"`
	Branch string `json:"branch"`
}

func ListWorktrees(ctx context.Context, repository string, logger *slog.Logger) ([]Worktree, error) {
	args := []string{
		"-C", repository, "worktree", "list", "--porcelain",
	}
	cmd := exec.CommandContext(ctx, "git", args...)

	log := logger.WithGroup("git").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&buf, w)
	cmd.Stderr = w

	log.Info("git worktree list", slog.String("repository", repository))
	if err := cmd.Run(); err != nil {
		log.Error("git worktree list", slog.Any("error", err))
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var wts []Worktree
	var cur *Worktree
	sc := bufio.NewScanner(&buf)
	for sc.Scan() {
		line := sc.Text()
		if line == "" { // blank line ends a record
			if cur != nil {
				wts = append(wts, *cur)
				cur = nil
			}
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		if cur == nil {
			cur = &Worktree{}
		}
		switch key {
		case "worktree":
			cur.Path = val
		case "HEAD":
			cur.Head = val
		case "branch":
			cur.Branch = strings.TrimPrefix(val, "refs/heads/")
		}
	}
	if cur != nil { // last record if no trailing blank line
		wts = append(wts, *cur)
	}
	return wts, sc.Err()
}

func AddWorktree(ctx context.Context, repository string, dir string, initialBranch string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "worktree", "add", "-b", initialBranch, dir,
	}
	cmd := exec.CommandContext(ctx, "git", args...)

	log := logger.WithGroup("git").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	log.Info(
		"git worktree add",
		slog.String("repository", repository), slog.String("dir", dir), slog.String("branch", initialBranch),
	)
	if err := cmd.Run(); err != nil {
		log.Error("git worktree add", slog.Any("error", err))
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

func RemoveWorktree(ctx context.Context, repository string, dir string, initialBranch string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "worktree", "remove", dir,
	}
	cmd := exec.CommandContext(ctx, "git", args...)

	log := logger.WithGroup("git").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	log.Info("git worktree remove", slog.String("repository", repository), slog.String("dir", dir))
	if err := cmd.Run(); err != nil {
		log.Error("git worktree remove", slog.Any("error", err))
		return fmt.Errorf("git worktree remove: %w", err)
	}

	// delete initial branch, if it is not found - just ignore error
	_ = RemoveBranchForce(ctx, repository, initialBranch, logger)
	return nil
}

func MoveWorktree(ctx context.Context, repository string, oldPath, newPath string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "worktree", "move", oldPath, newPath,
	}
	cmd := exec.CommandContext(ctx, "git", args...)

	log := logger.WithGroup("git").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	log.Info("git worktree move", slog.String("repository", repository), slog.String("oldPath", oldPath), slog.String("newPath", newPath))
	if err := cmd.Run(); err != nil {
		log.Error("git worktree remove", slog.Any("error", err))
		return fmt.Errorf("git worktree move: %w", err)
	}
	return nil
}
