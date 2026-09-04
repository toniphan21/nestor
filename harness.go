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

	FillProfileDefaultValues(runtime Runtime, profile *Profile)

	Syncers(runtime Runtime, profile Profile) []Syncer

	Init(runtime Runtime) error

	Mounts(runtime Runtime, profile Profile, sandbox Sandbox) (map[string]SandboxMount, error)

	Env(runtime Runtime, profile Profile) map[string]string

	ExecCommand(runtime Runtime, profile Profile, req ExecRequest) []string
}
