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

type BuildOption struct {
	Target     string
	Tag        string
	Dockerfile string
}

func Build(ctx context.Context, path string, options BuildOption, logger *slog.Logger) (string, error) {
	iidPath := filepath.Join(path, ".iid")

	args := []string{"build", "--iidfile", iidPath}
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

	log := logger.WithGroup("docker").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = w

	log.Info("build start")
	if err := cmd.Run(); err != nil {
		log.Error("build error", slog.Any("error", err))
		return "", fmt.Errorf("docker build: %w", err)
	}

	id, err := fs.AtomicReadFile(iidPath)
	if err != nil {
		log.Error("cannot read image id", slog.Any("error", err))
		return "", fmt.Errorf("docker build: read image id %q: %w", path, err)
	}

	log.Info("build finished")
	if err = fs.RemoveFile(iidPath); err != nil {
		log.Error("docker build: remove image id %q failed", slog.Any("error", err), slog.String("path", iidPath))
	}
	return strings.TrimSpace(string(id)), nil
}

type ImageInfo struct {
	ID     string
	Exists bool
}

func InspectImage(ctx context.Context, ref string, logger *slog.Logger) ImageInfo {
	args := []string{"image", "inspect", "--format", "{{.Id}}", "--", ref}
	cmd := exec.CommandContext(ctx, "docker", args...)

	log := logger.WithGroup("docker").With(slog.Any("args", args))

	// do not log stdout/stderr of docker inspect
	log.Info("docker image inspect")

	out, err := cmd.Output()
	if err != nil {
		log.Debug("docker image inspect return", slog.Bool("result", false))
		return ImageInfo{}
	}

	info := ImageInfo{
		ID:     strings.TrimSpace(string(out)),
		Exists: true,
	}
	log.Debug("docker image inspect return",
		slog.Bool("result", true),
		slog.String("id", info.ID),
	)
	return info
}
