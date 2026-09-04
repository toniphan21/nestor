package nestor

type harness string

const HarnessClaudeCode = harness("claude")
const HarnessOpenCode = harness("opencode")

type ExecRequest struct {
	PromptFilePath string
	Model          string
}

type Harness interface {
	Name() string

	DisplayName() string

	DefaultOptions(runtime Runtime) map[string]string

	DefaultDockerfile(runtime Runtime) string

	Init(runtime Runtime) error

	Syncers(sandbox Sandbox) []Syncer

	Mounts(sandbox Sandbox) (map[string]SandboxMount, error)

	Env(sandbox Sandbox) map[string]string

	ExecCommand(sandbox Sandbox, req ExecRequest) []string
}
