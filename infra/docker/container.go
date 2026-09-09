package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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
	result := len(strings.TrimSpace(out)) > 0
	log.Debug("docker ps return", slog.Bool("result", result), slog.String("container", container))
	return result
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

type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

func (m Mount) arg() string {
	if m.ReadOnly {
		return fmt.Sprintf("%s:%s:ro", m.Source, m.Target)
	}
	return fmt.Sprintf("%s:%s:rw", m.Source, m.Target)
}

type RunOption struct {
	HostAlias string
	Env       map[string]string
	Mounts    []Mount
}

func Run(ctx context.Context, image, name string, opt RunOption, logger *slog.Logger) (string, error) {
	loggedArgs := []string{"run", "--rm", "-d", "--name", name}
	args := []string{"run", "--rm", "-d", "--name", name}
	for k, v := range opt.Env {
		if k != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
			loggedArgs = append(loggedArgs, "-e", fmt.Sprintf("%s=redacted", k))
		}
	}

	for _, m := range opt.Mounts {
		args = append(args, "-v", m.arg())
		loggedArgs = append(args, "-v", m.arg())
	}

	if opt.HostAlias != "" {
		args = append(args, "--add-host", fmt.Sprintf("%s:host-gateway", opt.HostAlias))
		loggedArgs = append(args, "--add-host", fmt.Sprintf("%s:host-gateway", opt.HostAlias))
	}
	args = append(args, image)

	var stdout, stderr bytes.Buffer
	log := logger.WithGroup("docker").With(slog.Any("args", loggedArgs))
	w := &logWriter{log, slog.LevelDebug}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = io.MultiWriter(w, &stdout)
	cmd.Stderr = io.MultiWriter(w, &stderr)

	loggedOpt := RunOption{
		HostAlias: opt.HostAlias,
		Mounts:    opt.Mounts,
	}
	if opt.Env != nil {
		loggedOpt.Env = make(map[string]string)
		for k, _ := range opt.Env {
			loggedOpt.Env[k] = "redacted"
		}
	}

	log.Info("docker run", slog.String("image", image), slog.String("name", name), slog.Any("opt", loggedOpt))
	if err := cmd.Run(); err != nil {
		log.Error("docker run", slog.Any("error", err))

		return "", fmt.Errorf("docker run %q: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

type ExecOption struct {
	Env     map[string]string
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
}

func Exec(ctx context.Context, container string, commands []string, opt ExecOption, logger *slog.Logger) (int, error) {
	args := []string{
		"exec",
	}
	if opt.WorkDir != "" {
		args = append(args, "--workdir", opt.WorkDir)
	}
	for k, v := range opt.Env {
		if k != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}
	args = append(args, container)
	args = append(args, commands...)

	log := logger.WithGroup("docker").With(slog.Any("args", args))
	w := &logWriter{log, slog.LevelDebug}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = io.MultiWriter(w, opt.Stdout)
	cmd.Stderr = opt.Stderr

	log.Info("docker exec", slog.Any("args", args))
	if err := cmd.Start(); err != nil {
		log.Error("docker exec", slog.Any("error", err))
		return -1, fmt.Errorf("docker exec: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return ee.ExitCode(), nil // command failed; not a nestor failure
		}
		return 0, err

	case <-ctx.Done():
		<-done
		return -1, ctx.Err()
	}
}

func ExecInteractive(ctx context.Context, container string, commands []string, opt ExecOption, logger *slog.Logger) (int, error) {
	args := []string{"exec", "-it"}
	if opt.WorkDir != "" {
		args = append(args, "--workdir", opt.WorkDir)
	}
	for k, v := range opt.Env {
		if k != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}
	args = append(args, container)
	args = append(args, commands...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log := logger.WithGroup("docker").With(slog.Any("args", args))
	log.Info("docker exec -it", slog.Any("args", args))

	// Ctrl-C goes to the whole foreground process group, so docker gets its
	// own copy straight from the tty. Ignore ours so the harness handles it.
	signal.Ignore(syscall.SIGINT, syscall.SIGQUIT)
	defer signal.Reset(syscall.SIGINT, syscall.SIGQUIT)

	// SIGTERM/SIGHUP are sent to us alone (kill, terminal closed).
	// Forward so docker doesn't outlive us.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-sigs:
			cmd.Process.Signal(syscall.SIGTERM)
		case <-done:
		}
	}()

	err := cmd.Wait()
	close(done)

	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return ee.ExitCode(), nil // command failed; not a nestor failure
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}
