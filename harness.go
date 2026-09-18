package nestor

import (
	"net/http"
	"time"
)

type harness string

const HarnessClaudeCode = harness("claude")
const HarnessOpenCode = harness("opencode")

type ExecRequest struct {
	Interactive    bool
	PromptFilePath string
	Model          string
	SessionID      string
	Title          string
}

type Harness interface {
	Name() string

	DisplayName() string

	DefaultOptions(runtime Runtime) map[string]string

	DefaultDockerfile(runtime Runtime) string

	Init(runtime Runtime) error

	Syncers(sandbox Sandbox) []Syncer

	Mounts(sandbox Sandbox) (map[string]SandboxMount, error)

	StartEnv(sandbox Sandbox) map[string]string

	Exec(lease *Lease, req ExecRequest) HarnessExec

	CaptureSessionID(line []byte) (string, bool)

	ProxyRoute(lease *Lease) *ProxyRoute

	ListSessions(lease *Lease) []HarnessSession
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
