package docker

import (
	"bytes"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

func IsRunning(container string, logger *slog.Logger) bool {
	args := []string{
		"ps", "-q", "-f", "name=^" + container + "$",
	}
	cmd := exec.Command("docker", args...)

	log := logger.WithGroup("run").With(slog.String("cmd", "docker"), slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&buf, w)
	cmd.Stderr = w

	logger.Info("docker ps", slog.String("container", container))
	if err := cmd.Run(); err != nil {
		logger.Error("docker ps", slog.Any("error", err))
		return false
	}

	out := buf.String()
	return len(strings.TrimSpace(out)) > 0
}

func Kill(container string, logger *slog.Logger) error {
	args := []string{"kill", container}
	cmd := exec.Command("docker", args...)

	log := logger.WithGroup("run").With(slog.String("cmd", "docker"), slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd.Run()
}
