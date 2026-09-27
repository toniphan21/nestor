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

	"nhatp.com/go/nestor/internal/nx"
)

func IsRunning(ctx context.Context, container string, logger *slog.Logger) bool {
	var args nx.ArgsBuilder
	args.Add("ps", "-q", "-f", "name=^"+container+"$")

	cmd := args.ToCommandContext(ctx, "docker")

	log := args.Log(logger, "docker")
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
	var args nx.ArgsBuilder
	secs := max(int(timeout.Round(time.Second).Seconds()), 0)

	args.Add("stop", "-t", strconv.Itoa(secs), container)
	cmd := args.ToCommandContext(ctx, "docker")

	var stderr bytes.Buffer

	log := args.Log(logger, "docker")
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
	var args nx.ArgsBuilder
	args.Add("run", "--rm", "-d", "--name", name)
	for k, v := range opt.Env {
		if k == "" {
			continue
		}
		args.Add("-e").AddPair(fmt.Sprintf("%s=%s", k, v), fmt.Sprintf("%s=redacted", k))
	}

	for _, m := range opt.Mounts {
		args.Add("-v", m.arg())
	}

	if opt.HostAlias != "" {
		args.Add("--add-host", fmt.Sprintf("%s:host-gateway", opt.HostAlias))
	}
	args.Add(image)

	var stdout, stderr bytes.Buffer
	log := args.Log(logger, "docker")
	w := &logWriter{log, slog.LevelDebug}

	cmd := args.ToCommandContext(ctx, "docker")
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

func (o *ExecOption) fillArgs(args *nx.ArgsBuilder) {
	if o.WorkDir != "" {
		args.Add("--workdir", o.WorkDir)
	}
	for k, v := range o.Env {
		if k == "" {
			continue
		}
		args.Add("-e").AddPair(fmt.Sprintf("%s=%s", k, v), fmt.Sprintf("%s=redacted", k))
	}
}

func Exec(ctx context.Context, container string, commands []string, opt ExecOption, logger *slog.Logger) (int, error) {
	var args nx.ArgsBuilder
	args.Add("exec")

	opt.fillArgs(&args)

	args.Add(container)
	args.Add(commands...)

	log := args.Log(logger, "docker")
	w := &logWriter{log, slog.LevelDebug}

	cmd := args.ToCommandContext(ctx, "docker")
	cmd.Stdout = io.MultiWriter(w, opt.Stdout)
	cmd.Stderr = opt.Stderr

	log.Info("docker exec")
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
	var args nx.ArgsBuilder
	args.Add("exec", "-it")

	opt.fillArgs(&args)

	args.Add(container)
	args.Add(commands...)

	cmd := args.ToCommandContext(ctx, "docker")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log := args.Log(logger, "docker")
	log.Info("docker exec -it")

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
