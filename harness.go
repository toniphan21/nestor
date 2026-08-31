package nestor

import (
	"log/slog"
)

type harness string

const HarnessClaudeCode = harness("claude")
const HarnessOpenCode = harness("opencode")

type Harness interface {
	Name() string

	FillProfileDefaultValues(ctx Context, profile *Profile)

	Init(ctx Context, dir string, log *slog.Logger) error
}
