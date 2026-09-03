package nestor

import (
	"context"
)

type harness string

const HarnessClaudeCode = harness("claude")
const HarnessOpenCode = harness("opencode")

type Harness interface {
	Name() string

	DisplayName() string

	FillProfileDefaultValues(runtime Runtime, profile *Profile)

	Syncers(runtime Runtime, profile Profile) []Syncer

	Init(runtime Runtime) error

	Mounts(runtime Runtime, profile Profile, sandbox Sandbox) (map[string]SandboxMount, error)

	ExecCommand(ctx context.Context, promptPath string, profile Profile, model string) []string
}
