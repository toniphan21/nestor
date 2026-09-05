package nestor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
	"nhatp.com/go/nestor/infra/fs"
)

var errUnknownSandbox = errors.New("unknown sandbox implementation, use default one, don't implement Sandbox")

type Lease struct {
	data    leaseData
	sandbox Sandbox
	log     *slog.Logger
}

func (l *Lease) ID() string {
	return l.data.ID
}

func (l *Lease) Sandbox() Sandbox {
	return l.sandbox
}

func (l *Lease) HostPath() string {
	return l.data.Path
}

func (l *Lease) WorkDir() string {
	return l.data.WorkDir
}

func (l *Lease) ExpiresAt() time.Time {
	return l.data.ExpiresAt
}

func (l *Lease) Extend(ctx context.Context) error {
	sandbox, ld, err := l.findLeaseData()
	if err != nil {
		return err
	}

	ld.ExpiresAt = time.Now().Add(sandbox.spec.LeaseExtendDuration())
	sandbox.data.Leases[ld.Path] = *ld
	l.log.Info("extend lease",
		slog.String("sandbox", sandbox.ID()),
		slog.String("leaseId", l.ID()),
	)
	return sandbox.save(ctx)
}

func (l *Lease) Release(ctx context.Context) error {
	sandbox, ld, err := l.findLeaseData()
	if err != nil {
		return err
	}

	delete(sandbox.data.Leases, ld.Path)

	l.log.Info("release lease",
		slog.String("sandbox", sandbox.ID()),
		slog.String("leaseId", l.ID()),
	)
	return sandbox.save(ctx)
}

func (l *Lease) Run(ctx context.Context, prompt string, opt RunOption) (RunResult, error) {
	start := time.Now()
	sandbox, ok := l.sandbox.(*sandboxImpl)
	if !ok {
		return RunResult{ExitCode: -1}, errUnknownSandbox
	}

	if !sandbox.IsRunning(ctx) {
		if err := l.sandbox.Start(ctx); err != nil {
			return RunResult{ExitCode: -1}, err
		}
	}

	run := leaseRun{
		ID:      xid.New().String(),
		Model:   opt.Model,
		Stdout:  opt.Stdout != nil,
		Stderr:  opt.Stderr != nil,
		StartAt: start,
	}

	// update lease metadata
	date := l.data.CreatedAt.UTC().Format("2006-01-02")
	dir := sandbox.Dir("runs", date, l.ID())
	if err := fs.MkdirAll(dir); err != nil {
		return RunResult{ExitCode: -1}, fmt.Errorf("nestor: cannot make lease dir: %w", err)
	}

	runDir := filepath.Join(dir, run.ID)
	if err := fs.MkdirAll(runDir); err != nil {
		return RunResult{ExitCode: -1}, fmt.Errorf("nestor: cannot make lease run dir: %w", err)
	}

	metaFP := filepath.Join(dir, "meta.yml")
	meta, err := l.load(metaFP)
	if err != nil {
		return RunResult{ExitCode: -1}, fmt.Errorf("nestor: cannot read lease meta file: %w", err)
	}

	if err = fs.AtomicWriteFile(filepath.Join(runDir, "prompt"), []byte(prompt)); err != nil {
		return RunResult{ExitCode: -1}, fmt.Errorf("nestor: cannot write run prompt file: %w", err)
	}

	promptTargetPath := filepath.Join(SandboxRunsTargetPath, date, l.ID(), run.ID, "prompt")
	// collect harness cmd
	harnessCmd := sandbox.harness.ExecCommand(sandbox, ExecRequest{
		PromptFilePath: promptTargetPath,
		Model:          run.Model,
	})
	cmd := []string{
		"sh", "-c",
		strings.Join(harnessCmd, " "),
	}

	// execute the prompt
	stdout := bufferedFile{}
	stderr := bufferedFile{}

	l.log.Info("run lease",
		slog.String("sandbox", sandbox.ID()),
		slog.String("leaseId", l.ID()),
		slog.String("runId", run.ID),
	)
	code, err := sandbox.docker.Exec(ctx, sandbox.Container(), cmd, DockerExecOption{
		WorkDir: l.data.WorkDir,
		Stdout:  multiWriter(opt.Stdout, &stdout),
		Stderr:  multiWriter(opt.Stderr, &stderr),
	})

	if _, err = stdout.Save(filepath.Join(runDir, "stdout")); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}
	if _, err = stderr.Save(filepath.Join(runDir, "stderr")); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}

	run.Command = cmd
	run.EndAt = time.Now().UTC()

	meta.Run = append(meta.Run, run)
	if err = l.save(metaFP, meta); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}
	return RunResult{ExitCode: code, Duration: time.Since(start)}, err
}

func (l *Lease) findLeaseData() (*sandboxImpl, *leaseData, error) {
	sandbox, ok := l.sandbox.(*sandboxImpl)
	if !ok {
		return nil, nil, errUnknownSandbox
	}

	if sandbox.data.Leases == nil {
		return nil, nil, fmt.Errorf("%w: lease does not exist", ErrNotFound)
	}

	ld, ok := sandbox.data.Leases[l.data.Path]
	if !ok {
		return nil, nil, fmt.Errorf("%w: lease does not exist", ErrNotFound)
	}

	if ld.ID != l.data.ID {
		return nil, nil, fmt.Errorf("%w: lease id does not match host path", ErrNotFound)
	}

	if ld.ExpiresAt.Before(time.Now()) {
		return nil, nil, ErrLeaseExpired
	}
	return sandbox, &ld, nil
}

func (l *Lease) load(fp string) (*leaseMeta, error) {
	if !fs.HasFile(fp) {
		return &leaseMeta{Data: l.data}, nil
	}

	b, err := fs.AtomicReadFile(fp)
	if err != nil {
		return nil, err
	}

	var meta *leaseMeta
	if err := yaml.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (l *Lease) save(fp string, meta *leaseMeta) error {
	b, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return fs.AtomicWriteFile(fp, b)
}

type RunOption struct {
	Model  string
	Stdout io.Writer
	Stderr io.Writer
}

type RunResult struct {
	ID       string
	ExitCode int
	Duration time.Duration
}

type leaseMeta struct {
	Data leaseData `yaml:"data"`
	Run  []leaseRun
}

type leaseRun struct {
	ID      string    `yaml:"id"`
	Model   string    `yaml:"model"`
	Command []string  `yaml:"command"`
	Stdout  bool      `yaml:"stdout"`
	Stderr  bool      `yaml:"stderr"`
	Code    int       `yaml:"code"`
	Error   *string   `yaml:"error,omitempty"`
	StartAt time.Time `yaml:"start_at"`
	EndAt   time.Time `yaml:"end_at"`
}
