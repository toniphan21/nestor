package nestor

import (
	"log/slog"
	"path/filepath"
)

func newHarnessClaude() Harness {
	return &harnessClaude{
		assets: embedAssets{
			"assets/claude.dockerfile": {dst: "Dockerfile", file: true},
		},
	}
}

type harnessClaude struct {
	assets embedAssets
}

func (h *harnessClaude) Name() string {
	return string(HarnessClaudeCode)
}

func (h *harnessClaude) FillProfileDefaultValues(runtime Runtime, profile *Profile) {
	if profile.Dockerfile == "" {
		profile.Dockerfile = runtime.Platform.NestorDir("claude", "Dockerfile")
	}
	if profile.Options == nil {
		profile.Options = make(map[string]string)
	}

	cf, have := profile.Options[".claude"]
	if !have || cf == "" {
		profile.Options[".claude"] = runtime.Platform.HarnessDefaultOption(h.Name(), ".claude")
	}

	cfj, have := profile.Options[".claude.json"]
	if !have || cfj == "" {
		profile.Options[".claude.json"] = runtime.Platform.HarnessDefaultOption(h.Name(), ".claude.json")
	}
}

func (h *harnessClaude) Init(runtime Runtime) error {
	cp := filepath.Join(runtime.Platform.NestorDir(), "claude")
	have, err := h.assets.save(cp)
	if err != nil {
		return err
	}

	if have {
		runtime.Logger.Info("claude harness already exists, skip", slog.String("path", cp))
	} else {
		runtime.Logger.Info("saved builtin harness claude", slog.String("path", cp))
	}
	return nil
}

func (h *harnessClaude) Syncers(runtime Runtime, profile Profile) []Syncer {
	return nil
}

var _ Harness = (*harnessClaude)(nil)
