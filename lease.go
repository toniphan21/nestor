package nestor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
	"nhatp.com/go/nestor/infra/fs"
)

type leaseAPI interface {
	ID() string
	Sandbox() Sandbox
	HostPath() string
	WorkDir() string
	ProxyAddr() string
	ExpiresAt() time.Time
	Extend(ctx context.Context) error
	Release(ctx context.Context) error
	Run(ctx context.Context, prompt string, opt RunOption) (RunResult, error)
	SessionID() string
}

var errUnknownSandbox = errors.New("unknown sandbox implementation, use default one, don't implement Sandbox")

var _ leaseAPI = (*Lease)(nil)

type Lease struct {
	data      leaseData
	sandbox   Sandbox
	log       *slog.Logger
	sessionID string
	proxyAddr string
	proxySrv  *http.Server
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

func (l *Lease) ProxyAddr() string {
	return l.proxyAddr
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
	l.data.ExpiresAt = ld.ExpiresAt

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

	err = l.stopProxy()
	if err != nil {
		l.log.Warn("failed to stop proxy", "err", err)
	}
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
		ID:             xid.New().String(),
		WorkDir:        l.data.WorkDir,
		ModelRequested: opt.Model,
		Stdout:         opt.Stdout != nil,
		Stderr:         opt.Stderr != nil,
		StartAt:        start,
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
	l.sessionID = meta.SessionID

	if err = fs.AtomicWriteFile(filepath.Join(runDir, "prompt"), []byte(prompt), 0644); err != nil {
		return RunResult{ExitCode: -1}, fmt.Errorf("nestor: cannot write run prompt file: %w", err)
	}

	promptTargetPath := filepath.Join(SandboxRunsTargetPath, date, l.ID(), run.ID, "prompt")
	req := ExecRequest{
		PromptFilePath: promptTargetPath,
		Model:          run.ModelRequested,
		SessionID:      meta.SessionID,
	}

	if proxyRoute := sandbox.harness.ProxyRoute(l, req); proxyRoute != nil {
		if err = l.runProxy(proxyRoute); err != nil {
			return RunResult{ExitCode: -1}, fmt.Errorf("nestor: cannot start proxy: %w", err)
		}
	}

	// collect harness exec info
	he := sandbox.harness.Exec(l, req)
	cmd := []string{
		"sh", "-c",
		strings.Join(he.Command, " "),
	}

	// execute the prompt
	sessCapturer := newSessionIDCapture(sandbox.harness.CaptureSessionID)
	stdout := bufferedFile{}
	stderr := bufferedFile{}

	l.log.Info("run lease",
		slog.String("sandbox", sandbox.ID()),
		slog.String("leaseId", l.ID()),
		slog.String("runId", run.ID),
	)
	code, err := sandbox.docker.Exec(ctx, sandbox.Container(), cmd, DockerExecOption{
		Env:     he.Env,
		WorkDir: run.WorkDir,
		Stdout:  multiWriter(opt.Stdout, &stdout, sessCapturer),
		Stderr:  multiWriter(opt.Stderr, &stderr),
	})

	if _, err = stdout.Save(filepath.Join(runDir, "stdout")); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}
	if _, err = stderr.Save(filepath.Join(runDir, "stderr")); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}

	// update run and meta
	run.SessionID = he.SessionID
	run.Command = he.Command
	run.ModelUsed = he.Model
	run.EndAt = time.Now().UTC()

	meta.SessionID = sessCapturer.SessionID()
	l.sessionID = meta.SessionID

	meta.Run = append(meta.Run, run)
	meta.Data.ExpiresAt = l.ExpiresAt()

	if err = l.save(metaFP, meta); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}
	return RunResult{ExitCode: code, Duration: time.Since(start)}, err
}

func (l *Lease) SessionID() string {
	return l.sessionID
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
	return fs.AtomicWriteFile(fp, b, 0644)
}

func (l *Lease) runProxy(route *ProxyRoute) error {
	target, err := url.Parse(route.Target)
	if err != nil {
		return fmt.Errorf("parse proxy target %q: %w", route.Target, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("%w: proxy target %q is not absolute", ErrInvalid, route.Target)
	}

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return fmt.Errorf("listen for proxy: %w", err)
	}

	log := l.log.With("component", "proxy")

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
			route.Apply(r.Out.Header)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error("upstream failed", "path", r.URL.Path, "err", err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	srv := &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
	}
	l.proxySrv = srv

	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Error("proxy stopped", "err", err)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	l.proxyAddr = net.JoinHostPort(DefaultHostAlias, strconv.Itoa(port))
	log.Info("proxy started", "addr", l.proxyAddr, "target", target.Host)

	return nil
}

func (l *Lease) stopProxy() error {
	if l.proxySrv == nil {
		return nil
	}

	l.log.Info("proxy stop", "addr", l.proxyAddr)
	srv := l.proxySrv
	l.proxySrv = nil
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
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
	Data      leaseData `yaml:"data"`
	SessionID string    `yaml:"session_id"`
	Run       []leaseRun
}

type leaseRun struct {
	ID             string    `yaml:"id"`
	WorkDir        string    `yaml:"work_dir"`
	ModelRequested string    `yaml:"model_requested"`
	ModelUsed      string    `yaml:"model_used"`
	Command        []string  `yaml:"command"`
	SessionID      string    `yaml:"session_id"`
	Stdout         bool      `yaml:"stdout"`
	Stderr         bool      `yaml:"stderr"`
	Code           int       `yaml:"code"`
	Error          *string   `yaml:"error,omitempty"`
	StartAt        time.Time `yaml:"start_at"`
	EndAt          time.Time `yaml:"end_at"`
}
