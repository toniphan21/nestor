package nestor

import (
	"fmt"
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
	assets      embedAssets
	specManager *registry
}

func (h *harnessClaude) Name() string {
	return string(HarnessClaudeCode)
}

func (h *harnessClaude) fillDefaultValues(ctx Context, spec *HarnessSpec) {
	if spec.Dockerfile == "" {
		spec.Dockerfile = ctx.Platform().NestorDir("claude", "Dockerfile")
	}
	if spec.Options == nil {
		spec.Options = make(map[string]string)
	}

	cf, have := spec.Options[".claude"]
	if !have || cf == "" {
		spec.Options[".claude"] = ctx.Platform().HarnessDefaultOption(h.Name(), ".claude")
	}

	cfj, have := spec.Options[".claude.json"]
	if !have || cfj == "" {
		spec.Options[".claude.json"] = ctx.Platform().HarnessDefaultOption(h.Name(), ".claude.json")
	}
}

func (h *harnessClaude) Spec(ctx Context) (HarnessSpec, error) {
	s, have := ctx.Registry().HarnessSpec(h.Name())
	if have {
		h.fillDefaultValues(ctx, &s)
		return s, nil
	}

	return HarnessSpec{}, fmt.Errorf("%w: harness spec %q", ErrNotFound, h.Name())
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
