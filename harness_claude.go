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

func (h *harnessClaude) FillProfileDefaultValues(ctx Context, profile *Profile) {
	if profile.Dockerfile == "" {
		profile.Dockerfile = ctx.Platform().NestorDir("claude", "Dockerfile")
	}
	if profile.Options == nil {
		profile.Options = make(map[string]string)
	}

	cf, have := profile.Options[".claude"]
	if !have || cf == "" {
		profile.Options[".claude"] = ctx.Platform().HarnessDefaultOption(h.Name(), ".claude")
	}

	cfj, have := profile.Options[".claude.json"]
	if !have || cfj == "" {
		profile.Options[".claude.json"] = ctx.Platform().HarnessDefaultOption(h.Name(), ".claude.json")
	}
}

func (h *harnessClaude) Init(ctx Context, dir string, log *slog.Logger) error {
	cp := filepath.Join(dir, "claude")
	have, err := h.assets.save(cp)
	if err != nil {
		return err
	}

	if have {
		log.Info("claude harness already exists, skip", slog.String("path", cp))
	} else {
		log.Info("saved builtin harness claude", slog.String("path", cp))
	}
	return nil
}

var _ Harness = (*harnessClaude)(nil)
