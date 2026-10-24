package nestor

import (
	"context"
	"net/http"
	"time"
)

type harness string

const (
	HarnessClaudeCode = harness("claude")
	HarnessOpenCode   = harness("opencode")
)

type ExecRequest struct {
	Interactive      bool
	PromptFilePath   string
	Model            string
	SessionID        string
	Title            string
	InstructionFiles InstructionFiles
}

type InstructionFiles struct {
	Paths        []string
	CombinedPath string
}

func (f *InstructionFiles) HasFiles() bool {
	return len(f.Paths) > 0 && f.CombinedPath != ""
}

type Harness interface {
	Name() string

	DisplayName() string

	Init(runtime Runtime) error

	ResolveProfile(runtime Runtime, profile Profile) (ResolvedProfile, error)

	Syncers(sandbox Sandbox) []Syncer

	Mounts(sandbox Sandbox) (map[string]SandboxMount, error)

	StartEnv(sandbox Sandbox) map[string]string

	Exec(lease *Lease, req ExecRequest) (HarnessExec, error)

	CaptureSessionID(line []byte) (string, bool)

	ProxyRoute(lease *Lease) *ProxyRoute

	ListSessions(ctx context.Context, path string, sandboxes ...Sandbox) []HarnessSession
}

type ProxyRoute struct {
	Target string
	Apply  func(h http.Header)
}

type HarnessExec struct {
	Env       map[string]string
	Command   []string
	Model     string
	SessionID string
}

type HarnessSession struct {
	ID        string
	Title     string
	ProjectID string
	Directory string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ResolvedProfile interface {
	Dockerfile() string

	Model(request string) string

	SetupString() string
}
