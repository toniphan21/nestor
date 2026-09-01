package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"
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

	log.Info("docker kill", slog.String("container", container))
	if err := cmd.Run(); err != nil {
		log.Error("docker kill", slog.Any("error", err))
		return err
	}
	return nil
}

func Stop(ctx context.Context, container string, timeout time.Duration, logger *slog.Logger) error {
	secs := int(timeout.Round(time.Second).Seconds())
	if secs < 0 {
		secs = 0
	}

	args := []string{"stop", "-t", strconv.Itoa(secs), container}
	cmd := exec.CommandContext(ctx, "docker", args...)

	var stderr bytes.Buffer
	log := logger.WithGroup("docker").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}
	cmd.Stdout = w
	cmd.Stderr = io.MultiWriter(&stderr, w)

	log.Info("docker stop", slog.String("container", container))
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if strings.Contains(msg, "No such container") {
			return nil // already gone
		}

		log.Error("docker stop", slog.Any("error", err))
		return fmt.Errorf("docker stop %q: %w: %s", container, err, strings.TrimSpace(msg))
	}
	return nil
}

type RunOptions struct {
	Mounts []string
}

func Run(ctx context.Context, image, name string, options RunOptions, logger *slog.Logger) (string, error) {
	args := []string{"run", "--rm", "-d", "--name", name}
	for _, m := range options.Mounts {
		args = append(args, "-v", m)
	}
	args = append(args, image)

	var stdout, stderr bytes.Buffer
	log := logger.WithGroup("docker").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = io.MultiWriter(w, &stdout)
	cmd.Stderr = io.MultiWriter(w, &stderr)

	log.Info("docker run", slog.String("image", image), slog.String("name", name), slog.Any("options", options))
	if err := cmd.Run(); err != nil {
		log.Error("docker run", slog.Any("error", err))

		return "", fmt.Errorf("docker run %q: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

//func Exec(ctx context.Context, container string, argv []string, out ExecIO) (int, error) {
//	cmd := exec.Command("docker", append([]string{"exec", "-i", container}, argv...)...)
//	cmd.Stdout = out.Stdout
//	cmd.Stderr = out.Stderr
//
//	if err := cmd.Start(); err != nil {
//		return -1, fmt.Errorf("%w: docker exec: %w", ErrExec, err)
//	}
//
//	done := make(chan error, 1)
//	go func() { done <- cmd.Wait() }()
//
//	select {
//	case err := <-done:
//		var ee *exec.ExitError
//		if errors.As(err, &ee) {
//			return ee.ExitCode(), nil // command failed; not a nestor failure
//		}
//		return 0, err
//
//	case <-ctx.Done():
//		// killing the client leaves the inner process alive
//		d.stop(context.WithoutCancel(ctx), container)
//		<-done
//		return -1, ctx.Err()
//	}
//}
