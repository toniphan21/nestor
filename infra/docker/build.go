package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"nhatp.com/go/nestor/infra/fs"
)

type logWriter struct {
	log   *slog.Logger
	level slog.Level
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.log.Log(context.Background(), w.level, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

type BuildOptions struct {
	Target     string
	Tag        string
	Dockerfile string
}

func Build(ctx context.Context, path string, options BuildOptions, logger *slog.Logger) (string, error) {
	iidPath := filepath.Join(path, ".iid")

	args := []string{"build", "--progress=plain", "--iidfile", iidPath}
	if options.Target != "" {
		args = append(args, "--target", options.Target)
	}
	if options.Tag != "" {
		args = append(args, "--tag", options.Tag)
	}
	if options.Dockerfile != "" {
		args = append(args, "--file", options.Dockerfile)
	}
	args = append(args, path)

	cmd := exec.CommandContext(ctx, "docker", args...)

	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	log := logger.WithGroup("run").With(
		slog.String("cmd", "docker"),
		slog.Any("args", args),
	)
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	logger.Info("docker build start")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}

	id, err := fs.AtomicReadFile(iidPath)
	if err != nil {
		return "", fmt.Errorf("docker build: read image id %q: %w", path, err)
	}

	logger.Info("docker build finished")
	if err = fs.RemoveFile(iidPath); err != nil {
		logger.Error("docker build: remove image id %q failed", slog.Any("error", err), slog.String("path", iidPath))
	}
	return strings.TrimSpace(string(id)), nil
}
