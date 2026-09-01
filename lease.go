package nestor

import (
	"context"
	"io"
	"time"
)

type Lease struct {
	id        string
	path      string
	expiresAt time.Time
	sandbox   Sandbox
}

func (l *Lease) Sandbox() Sandbox {
	return l.sandbox
}

func (l *Lease) Path() string {
	return l.path
}

func (l *Lease) ExpiresAt() time.Time {
	return l.expiresAt
}

func (l *Lease) Extend() error {
	return nil
}

func (l *Lease) Release() error {
	return nil
}

func (l *Lease) Run(ctx context.Context, prompt string, out ExecIO) (RunResult, error) {
	start := time.Now()

	if !l.sandbox.IsRunning(ctx) {
		if err := l.sandbox.Start(ctx); err != nil {
			return RunResult{ExitCode: -1}, err
		}
	}

	return RunResult{ExitCode: 0, Duration: time.Since(start)}, nil
}

type ExecIO struct {
	Stdout io.Writer
	Stderr io.Writer
}

type RunResult struct {
	ExitCode int
	Duration time.Duration
}
