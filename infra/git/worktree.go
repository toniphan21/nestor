package git

import (
	"bufio"
	"bytes"
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

func ListWorktrees(repository string, logger *slog.Logger) ([]Worktree, error) {
	args := []string{
		"-C", repository, "worktree", "list", "--porcelain",
	}
	cmd := exec.Command("git", args...)

	log := logger.WithGroup("run").With(slog.String("cmd", "git"), slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&buf, w)
	cmd.Stderr = w

	logger.Info("git worktree list", slog.String("repository", repository))
	if err := cmd.Run(); err != nil {
		logger.Error("git worktree list", slog.Any("error", err))
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

func AddWorktree(repository string, dir string, initialBranch string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "worktree", "add", "-b", initialBranch, dir,
	}
	cmd := exec.Command("git", args...)

	log := logger.WithGroup("run").With(slog.String("cmd", "git"), slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	logger.Info(
		"git worktree add",
		slog.String("repository", repository), slog.String("dir", dir), slog.String("branch", initialBranch),
	)
	if err := cmd.Run(); err != nil {
		logger.Error("git worktree add", slog.Any("error", err))
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

func RemoveWorktree(repository string, dir string, initialBranch string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "worktree", "remove", dir,
	}
	cmd := exec.Command("git", args...)

	log := logger.WithGroup("run").With(slog.String("cmd", "git"), slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	logger.Info("git worktree remove", slog.String("repository", repository), slog.String("dir", dir))
	if err := cmd.Run(); err != nil {
		logger.Error("git worktree remove", slog.Any("error", err))
		return fmt.Errorf("git worktree remove: %w", err)
	}

	// delete initial branch, if it is not found - just ignore error
	_ = RemoveBranchForce(repository, initialBranch, logger)
	return nil
}
