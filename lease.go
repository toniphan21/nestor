package nestor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var errUnknownSandbox = errors.New("unknown sandbox implementation, use default one, don't implement Sandbox")

type Lease struct {
	id        string
	hostPath  string
	workDir   string
	expiresAt time.Time
	sandbox   Sandbox
}

func (l *Lease) Sandbox() Sandbox {
	return l.sandbox
}

func (l *Lease) HostPath() string {
	return l.hostPath
}

func (l *Lease) WorkDir() string {
	return l.workDir
}

func (l *Lease) ExpiresAt() time.Time {
	return l.expiresAt
}

func (l *Lease) Extend(ctx context.Context) error {
	sandbox, ld, err := l.findLeaseData()
	if err != nil {
		return err
	}

	ld.ExpiresAt = time.Now().Add(DefaultLeaseExtendDuration)
	sandbox.data.Leases[ld.Path] = *ld
	return sandbox.save(ctx)
}

func (l *Lease) Release(ctx context.Context) error {
	sandbox, ld, err := l.findLeaseData()
	if err != nil {
		return err
	}

	delete(sandbox.data.Leases, ld.Path)

	return sandbox.save(ctx)
}

func (l *Lease) Run(ctx context.Context, prompt string, out ExecIO) (RunResult, error) {
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

	harnessCmd := []string{
		"claude",
		"--dangerously-skip-permissions",
		"--output-format stream-json",
		"--verbose",
	}
	//if model != "" {
	//	cmd = append(cmd, "--model", model)
	//}
	harnessCmd = append(harnessCmd, "--print")
	harnessCmd = append(harnessCmd, "'tell me about golang'")

	cmd := []string{
		"sh", "-c",
		strings.Join(harnessCmd, " "),
	}

	var stdout bytes.Buffer
	code, err := sandbox.docker.Exec(ctx, sandbox.Container(), cmd, DockerExecOption{
		WorkDir: l.workDir,
		Stdout:  &stdout,
		Stderr:  out.Stderr,
	})
	fmt.Println(stdout.String())

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

	ld, ok := sandbox.data.Leases[l.hostPath]
	if !ok {
		return nil, nil, fmt.Errorf("%w: lease does not exist", ErrNotFound)
	}

	if ld.ID != l.id {
		return nil, nil, fmt.Errorf("%w: lease id does not match host path", ErrNotFound)
	}

	if ld.ExpiresAt.Before(time.Now()) {
		return nil, nil, ErrLeaseExpired
	}
	return sandbox, &ld, nil
}

type ExecIO struct {
	Stdout io.Writer
	Stderr io.Writer
}

type RunResult struct {
	ExitCode int
	Duration time.Duration
}
