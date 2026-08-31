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

func RemoveBranchForce(repository, branch string, logger *slog.Logger) error {
	args := []string{
		"-C", repository, "branch", "-D", branch,
	}
	cmd := exec.Command("git", args...)

	log := logger.WithGroup("run").With(slog.String("cmd", "git"), slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	logger.Info("git branch -D", slog.String("repository", repository), slog.String("branch", branch))
	if err := cmd.Run(); err != nil {
		logger.Error("git branch -D", slog.Any("error", err))
		return fmt.Errorf("git branch -D: %w", err)
	}
	return nil
}
