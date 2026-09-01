package nestor

import (
	"context"
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

	ch, have := profile.Options["container-home-dir"]
	if !have || ch == "" {
		profile.Options["container-home-dir"] = "/home/agent"
	}
}

func (h *harnessClaude) Init(runtime Runtime) error {
	log := runtime.Logger
	cp := filepath.Join(runtime.Platform.NestorDir(), "claude")
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

func (h *harnessClaude) Syncers(runtime Runtime, profile Profile) []Syncer {
	return nil
}

func (h *harnessClaude) Mounts(runtime Runtime, profile Profile, sandbox Sandbox) (map[string]string, error) {
	// .claude and .claude.json is copied from profile options to the sandbox.Dir() in Syncer
	ch := profile.Options["container-home-dir"]
	if ch == "" {
		ch = "/"
	}
	mounts := map[string]string{
		sandbox.Dir(".claude"):      filepath.Join(ch, ".claude:rw"),
		sandbox.Dir(".claude.json"): filepath.Join(ch, ".claude.json:rw"),
	}
	return mounts, nil
}

var _ Harness = (*harnessClaude)(nil)

type harnessClaudeSyncer struct{}

func (h *harnessClaudeSyncer) String() string {
	//TODO implement me
	panic("implement me")
}

func (h *harnessClaudeSyncer) Sync(ctx context.Context, sandbox Sandbox) error {
	//TODO implement me
	panic("implement me")
}

var _ Syncer = (*harnessClaudeSyncer)(nil)
