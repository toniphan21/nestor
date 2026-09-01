package docker

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

func IsRunning(ctx context.Context, container string, logger *slog.Logger) bool {
	args := []string{
		"ps", "-q", "-f", "name=^" + container + "$",
	}
	cmd := exec.CommandContext(ctx, "docker", args...)

	log := logger.WithGroup("docker").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&buf, w)
	cmd.Stderr = w

	log.Info("docker ps", slog.String("container", container))
	if err := cmd.Run(); err != nil {
		log.Error("docker ps", slog.Any("error", err))
		return false
	}

	out := buf.String()
	return len(strings.TrimSpace(out)) > 0
}

func Kill(ctx context.Context, container string, logger *slog.Logger) error {
	args := []string{"kill", container}
	cmd := exec.CommandContext(ctx, "docker", args...)

	log := logger.WithGroup("docker").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd.Run()
}
