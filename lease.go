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

type RunParam interface {
	validate() error

	resumeSession() bool

	sessionID() string
}

type Headless struct {
	Prompt       string
	Model        string
	SessionID    string
	Title        string
	Instructions []string
	Stdout       io.Writer
	Stderr       io.Writer
}

func (h Headless) validate() error {
	if h.Prompt == "" {
		return fmt.Errorf("%w: prompt is missing", ErrInvalid)
	}
	return nil
}

func (h Headless) resumeSession() bool {
	return strings.TrimSpace(h.SessionID) != ""
}

func (h Headless) sessionID() string {
	return strings.TrimSpace(h.SessionID)
}

type Interactive struct {
	Model                string
	SessionID            string
	Title                string
	Instructions         []string
	OnInit               func(authProxy string, mcpProxy string)
	OnSessionEstablished func(sessionID string)
}

func (i Interactive) validate() error {
	return nil
}

func (i Interactive) resumeSession() bool {
	return strings.TrimSpace(i.SessionID) != ""
}

func (i Interactive) sessionID() string {
	return strings.TrimSpace(i.SessionID)
}

type leaseAPI interface {
	ID() string
	Sandbox() Sandbox
	RequestedPath() string
	WorkDir() string
	HostWorkDir() string
	AuthProxyAddr() string
	MCPProxyAddr() string
	ExpiresAt() time.Time
	Extend(ctx context.Context) error
	Release(ctx context.Context) error
	Run(ctx context.Context, param RunParam) (RunResult, error)
	SessionID() string
	ListSessions() []HarnessSession
}

var errUnknownSandbox = errors.New("unknown sandbox implementation, use default one, don't implement Sandbox")

var _ leaseAPI = (*Lease)(nil)

type Lease struct {
	data          leaseData
	sandbox       Sandbox
	log           *slog.Logger
	sessionID     string
	proxyAuthAddr string
	proxyAuthSrv  *http.Server
	proxyMCPAddr  string
	proxyMCPSrv   *http.Server
}

func (l *Lease) ID() string {
	return l.data.ID
}

func (l *Lease) Sandbox() Sandbox {
	return l.sandbox
}

func (l *Lease) RequestedPath() string {
	return l.data.Path
}

func (l *Lease) WorkDir() string {
	return l.data.WorkDir
}

func (l *Lease) HostWorkDir() string {
	return l.data.HostWorkDir
}

func (l *Lease) AuthProxyAddr() string {
	return l.proxyAuthAddr
}

func (l *Lease) MCPProxyAddr() string {
	return l.proxyMCPAddr
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

	err = l.stopProxies()
	if err != nil {
		l.log.Warn("failed to stop proxy", "err", err)
	}
	return sandbox.save(ctx)
}

const (
	RunExitCodeInvalidParam       = -1
	RunExitCodeUnknownSandbox     = -2
	RunExitCodeCannotStartSandbox = -3
	RunExitCodeInternalError      = -4
)

func (l *Lease) Run(ctx context.Context, param RunParam) (RunResult, error) {
	if param == nil {
		return RunResult{ExitCode: RunExitCodeInvalidParam}, fmt.Errorf("%w: param is nil", ErrInvalid)
	}

	if err := param.validate(); err != nil {
		return RunResult{ExitCode: RunExitCodeInvalidParam}, err
	}

	start := time.Now()
	sandbox, ok := l.sandbox.(*sandboxImpl)
	if !ok {
		return RunResult{ExitCode: RunExitCodeUnknownSandbox}, errUnknownSandbox
	}

	if !sandbox.IsRunning(ctx) {
		if err := l.sandbox.Start(ctx); err != nil {
			return RunResult{ExitCode: RunExitCodeCannotStartSandbox}, err
		}
	}

	if param.resumeSession() && !l.hasSession(param.sessionID()) {
		return RunResult{ExitCode: RunExitCodeInvalidParam}, fmt.Errorf("%w: session %q", ErrNotFound, param.sessionID())
	}

	run := leaseRun{
		ID:      xid.New().String(),
		WorkDir: l.data.WorkDir,
		StartAt: start,
	}

	// update lease metadata
	date := l.data.CreatedAt.UTC().Format("2006-01-02")
	dir := sandbox.Dir("runs", date, l.ID())
	if err := fs.MkdirAll(dir); err != nil {
		return RunResult{ExitCode: RunExitCodeInternalError}, fmt.Errorf("nestor: cannot make lease dir: %w", err)
	}

	runDir := filepath.Join(dir, run.ID)
	if err := fs.MkdirAll(runDir); err != nil {
		return RunResult{ExitCode: RunExitCodeInternalError}, fmt.Errorf("nestor: cannot make lease run dir: %w", err)
	}
	runDirInContainer := filepath.Join(SandboxRunsTargetPath, date, l.ID(), run.ID)

	metaFP := filepath.Join(dir, "meta.yml")
	meta, err := l.load(metaFP, param)
	if err != nil {
		return RunResult{ExitCode: RunExitCodeInternalError}, fmt.Errorf("nestor: cannot read lease meta file: %w", err)
	}

	l.sessionID = meta.SessionID

	// create auth proxy
	if proxyRoute := sandbox.harness.ProxyRoute(l); proxyRoute != nil {
		if err = l.runAuthProxy(proxyRoute); err != nil {
			return RunResult{ExitCode: RunExitCodeInternalError}, fmt.Errorf("nestor: cannot start auth proxy: %w", err)
		}
	}

	// create mcp proxy
	if err = l.runMCPProxy(sandbox.spec.MCPs); err != nil {
		return RunResult{ExitCode: RunExitCodeInternalError}, fmt.Errorf("nestor: cannot start mcp proxy: %w", err)
	}

	switch v := param.(type) {
	case Headless:
		l.save(metaFP, meta)
		code, err := l.runHeadless(ctx, sandbox, &v, meta, &run, runDir, runDirInContainer)
		l.save(metaFP, meta)
		return RunResult{ExitCode: code, Duration: time.Since(start)}, err

	case Interactive:
		if !param.resumeSession() {
			// Run a predefined prompt to confirm the harness is running inside the
			// container. This is intentional: on launch the user sees the confirmation
			// before the TUI opens. As a side effect, the headless run gives us the
			// session ID to resume with.
			if v.OnInit != nil {
				v.OnInit(l.proxyAuthAddr, l.proxyMCPAddr)
			}
			prompt := sandbox.runtime.Template.MakeInteractiveConfirmPrompt(l.proxyAuthAddr, l.proxyMCPAddr)
			l.save(metaFP, meta)
			_, _ = l.runHeadless(ctx, sandbox, &Headless{Prompt: prompt, Title: v.Title}, meta, &run, runDir, runDirInContainer)
			l.save(metaFP, meta)

			if v.OnSessionEstablished != nil {
				v.OnSessionEstablished(l.sessionID)
			}
		}

		code, err := l.runInteractive(ctx, sandbox, &v, meta, &run, runDir, runDirInContainer)
		return RunResult{ExitCode: code}, err

	default:
		return RunResult{ExitCode: RunExitCodeInvalidParam}, fmt.Errorf("%w: unknown param type", ErrInvalid)
	}
}

func (l *Lease) SessionID() string {
	return l.sessionID
}

func (l *Lease) ListSessions() []HarnessSession {
	return l.sandbox.Harness().ListSessions(l)
}

func (l *Lease) hasSession(id string) bool {
	seen := make(map[string]bool)
	for _, v := range l.ListSessions() {
		seen[v.ID] = true
	}
	return seen[id]
}

func (l *Lease) runHeadless(
	ctx context.Context,
	sandbox *sandboxImpl,
	param *Headless,
	meta *leaseMeta,
	run *leaseRun,
	runDir string,
	runDirInContainer string,
) (int, error) {
	run.ModelRequested = param.Model
	run.Stdout = param.Stdout != nil
	run.Stderr = param.Stderr != nil

	if err := fs.AtomicWriteFile(filepath.Join(runDir, "prompt"), []byte(param.Prompt), 0644); err != nil {
		return -1, fmt.Errorf("nestor: cannot write run prompt file: %w", err)
	}

	promptTargetPath := filepath.Join(runDirInContainer, "prompt")
	req := ExecRequest{
		Interactive:      false,
		PromptFilePath:   promptTargetPath,
		Model:            run.ModelRequested,
		SessionID:        meta.SessionID,
		InstructionFiles: l.makeInstructionFiles(runDir, runDirInContainer, param.Instructions),
	}
	if meta.SessionID == "" {
		req.Title = param.Title
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

	l.log.Info("run lease headless",
		slog.String("sandbox", sandbox.ID()),
		slog.String("leaseId", l.ID()),
		slog.String("runId", run.ID),
	)
	code, runErr := sandbox.docker.Exec(ctx, sandbox.Container(), cmd, DockerExecOption{
		Env:     he.Env,
		WorkDir: run.WorkDir,
		Stdout:  multiWriter(param.Stdout, &stdout, sessCapturer),
		Stderr:  multiWriter(param.Stderr, &stderr),
	})

	if _, err := stdout.Save(filepath.Join(runDir, "stdout")); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}
	if _, err := stderr.Save(filepath.Join(runDir, "stderr")); err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}

	// update run and meta
	run.SessionID = he.SessionID
	run.Command = he.Command
	run.ModelUsed = he.Model
	run.EndAt = time.Now().UTC()

	meta.SessionID = sessCapturer.SessionID()
	l.sessionID = meta.SessionID
	// save session to sandbox
	if err := sandbox.addSession(ctx, l.WorkDir(), l.sessionID); err != nil {
		l.log.Warn("cannot add session", slog.Any("error", err))
	}

	meta.Run = append(meta.Run, run)
	meta.Data.ExpiresAt = l.ExpiresAt()
	return code, runErr
}

func (l *Lease) runInteractive(
	ctx context.Context,
	sandbox *sandboxImpl,
	param *Interactive,
	meta *leaseMeta,
	run *leaseRun,
	runDir string,
	runDirInContainer string,
) (int, error) {
	run.Interactive = true
	run.ModelRequested = param.Model
	run.Stdout = false
	run.Stderr = false

	var sessionID string
	if param.resumeSession() {
		sessionID = param.SessionID
	} else {
		// if there is no SessionID in the param, resume from the initial prompt (meta.SessionID)
		sessionID = meta.SessionID
	}

	req := ExecRequest{
		Interactive:      true,
		Model:            run.ModelRequested,
		SessionID:        sessionID,
		InstructionFiles: l.makeInstructionFiles(runDir, runDirInContainer, param.Instructions),
	}
	if meta.SessionID == "" {
		req.Title = param.Title
	}

	// collect harness exec info
	he := sandbox.harness.Exec(l, req)

	// execute the prompt
	l.log.Info("run lease interactive",
		slog.String("sandbox", sandbox.ID()),
		slog.String("leaseId", l.ID()),
		slog.String("runId", run.ID),
	)
	code, runErr := sandbox.docker.ExecInteractive(ctx, sandbox.Container(), he.Command, DockerExecInteractiveOption{
		Env:     he.Env,
		WorkDir: run.WorkDir,
	})

	// update run and meta
	run.SessionID = he.SessionID
	run.Command = he.Command
	run.ModelUsed = he.Model
	run.EndAt = time.Now().UTC()

	meta.SessionID = l.sessionID
	// save session to sandbox
	if err := sandbox.addSession(ctx, l.WorkDir(), l.sessionID); err != nil {
		l.log.Warn("cannot add session", slog.Any("error", err))
	}

	meta.Run = append(meta.Run, run)
	meta.Data.ExpiresAt = l.ExpiresAt()
	return code, runErr
}

func (l *Lease) makeInstructionFiles(hostDir string, containerDir string, instructions []string) InstructionFiles {
	var out InstructionFiles
	if len(instructions) == 0 {
		return out
	}

	var sb strings.Builder

	os := l.Sandbox().Runtime().Platform.OS()
	for i, v := range instructions {
		vv := strings.TrimSpace(v)
		if vv == "" {
			continue
		}

		hp := filepath.Join(hostDir, fmt.Sprintf("instruction-%d", i))
		cp := filepath.Join(containerDir, fmt.Sprintf("instruction-%d", i))
		ins := parseInstruction(vv, os)

		if err := ins.saveTo(hp); err == nil {
			sb.WriteString(ins.content())
			sb.WriteString("\n\n")
			out.Paths = append(out.Paths, cp)
		}
	}

	if sb.Len() != 0 {
		hp := filepath.Join(hostDir, "instruction")
		cp := filepath.Join(containerDir, "instruction")

		if err := fs.WriteFile(hp, []byte(sb.String())); err == nil {
			out.CombinedPath = cp
		}
	}
	return out
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

func (l *Lease) load(fp string, param RunParam) (*leaseMeta, error) {
	if !fs.HasFile(fp) {
		return &leaseMeta{Data: l.data, SessionID: param.sessionID()}, nil
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

func (l *Lease) save(fp string, meta *leaseMeta) {
	b, err := yaml.Marshal(meta)
	if err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
		return
	}

	err = fs.AtomicWriteFile(fp, b, 0644)
	if err != nil {
		l.log.Warn("cannot save lease meta file", slog.Any("error", err))
	}
}

func (l *Lease) runAuthProxy(route *ProxyRoute) error {
	l.proxyAuthAddr = ""

	target, err := url.Parse(route.Target)
	if err != nil {
		return fmt.Errorf("parse proxy target %q: %w", route.Target, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("%w: proxy target %q is not absolute", ErrInvalid, route.Target)
	}

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return fmt.Errorf("listen for auth proxy: %w", err)
	}

	log := l.log.With("component", "auth-proxy")

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
	l.proxyAuthSrv = srv

	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Error("auth proxy stopped", "err", err)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	l.proxyAuthAddr = net.JoinHostPort(DefaultHostAlias, strconv.Itoa(port))
	log.Info("auth proxy started", "addr", l.proxyAuthAddr, "target", target.Host)

	return nil
}

func (l *Lease) runMCPProxy(mcps []string) error {
	addr, server, err := runMCPProxy(l.sandbox.Runtime(), mcps)
	l.proxyMCPAddr = addr
	l.proxyMCPSrv = server

	return err
}

func (l *Lease) stopProxies() error {
	if err := l.stopAuthProxy(); err != nil {
		return err
	}
	if err := l.stopMCPProxy(); err != nil {
		return err
	}
	return nil
}

func (l *Lease) stopAuthProxy() error {
	if l.proxyAuthSrv == nil {
		return nil
	}

	l.log.Info("auth proxy stop", "addr", l.proxyAuthAddr)
	srv := l.proxyAuthSrv
	l.proxyAuthSrv = nil
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func (l *Lease) stopMCPProxy() error {
	if l.proxyMCPSrv == nil {
		return nil
	}

	l.log.Info("MCP proxy stop", "addr", l.proxyMCPAddr)
	srv := l.proxyMCPSrv
	l.proxyMCPSrv = nil
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
	Run       []*leaseRun
}

type leaseRun struct {
	ID             string    `yaml:"id"`
	WorkDir        string    `yaml:"work_dir"`
	ModelRequested string    `yaml:"model_requested"`
	Interactive    bool      `yaml:"interactive"`
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
