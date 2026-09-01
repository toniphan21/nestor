package nestor

type harness string

const HarnessClaudeCode = harness("claude")
const HarnessOpenCode = harness("opencode")

type Harness interface {
	Name() string

	FillProfileDefaultValues(runtime Runtime, profile *Profile)

	Syncers(runtime Runtime, profile Profile) []Syncer

	Init(runtime Runtime) error
}
