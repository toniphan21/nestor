package nestor

import "net/http"

type harness string

const HarnessClaudeCode = harness("claude")
const HarnessOpenCode = harness("opencode")

type ExecRequest struct {
	PromptFilePath string
	Model          string
	SessionID      string
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

	ExecEnv(lease *Lease, req ExecRequest) map[string]string

	ExecCommand(lease *Lease, req ExecRequest) []string

	CaptureSessionID(line []byte) (string, bool)

	ProxyRoute(lease *Lease, req ExecRequest) *ProxyRoute
}

type ProxyRoute struct {
	Target string
	Apply  func(h http.Header)
}
