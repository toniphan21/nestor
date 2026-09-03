package nestor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"

	"nhatp.com/go/nestor/infra/fs"
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
	s := &harnessClaudeSyncer{
		auth:       profile.Auth,
		claudeDir:  profile.Options[".claude"],
		claudeJson: profile.Options[".claude.json"],
		os:         runtime.Platform.OS(),
	}
	return []Syncer{s}
}

func (h *harnessClaude) Mounts(runtime Runtime, profile Profile, sandbox Sandbox) (map[string]SandboxMount, error) {
	// .claude and .claude.json is copied from profile options to the sandbox.Dir() in Syncer
	ch := profile.Options["container-home-dir"]
	if ch == "" {
		ch = "/"
	}

	cd := sandbox.Dir(".claude")
	cj := sandbox.Dir(".claude.json")
	mounts := map[string]SandboxMount{
		cd: {Host: cd, Target: filepath.Join(ch, ".claude")},
		cj: {Host: cj, Target: filepath.Join(ch, ".claude.json")},
	}
	return mounts, nil
}

var _ Harness = (*harnessClaude)(nil)

type harnessClaudeSyncer struct {
	auth       auth
	claudeDir  string
	claudeJson string
	os         OSKind
}

func (h *harnessClaudeSyncer) Sync(ctx context.Context, sandbox Sandbox) error {
	targetDir := sandbox.Dir()
	var err error
	err = fs.MergeDir(h.claudeDir, filepath.Join(targetDir, ".claude"))
	if err != nil {
		return err
	}

	err = fs.CopyFile(h.claudeJson, filepath.Join(targetDir, ".claude.json"))
	if err != nil {
		return err
	}

	if h.auth == AuthCredentials && h.os == OSMacOS {
		cc, err := loadClaudeCredentials(ctx)
		if err != nil {
			return err
		}
		if cc.OAuth.RefreshTokenExpired(1 * time.Hour) {
			return errors.New("refresh token expired")
		}

		dst := filepath.Join(targetDir, ".claude", ".credentials.json")
		return cc.Save(dst)
	}
	return nil
}

var _ Syncer = (*harnessClaudeSyncer)(nil)

type harnessClaudeCredentials struct {
	OAuth harnessClaudeOAuth `json:"claudeAiOauth"`
}

type harnessClaudeOAuth struct {
	AccessToken           string   `json:"accessToken"`
	RefreshToken          string   `json:"refreshToken"`
	ExpiresAt             int64    `json:"expiresAt"`             // unix millis
	RefreshTokenExpiresAt int64    `json:"refreshTokenExpiresAt"` // unix millis
	Scopes                []string `json:"scopes"`
	SubscriptionType      string   `json:"subscriptionType"`
	RateLimitTier         string   `json:"rateLimitTier"`
}

func (o harnessClaudeOAuth) RefreshTokenExpiry() time.Time {
	if o.RefreshTokenExpiresAt == 0 {
		return time.Time{}
	}
	return time.UnixMilli(o.RefreshTokenExpiresAt)
}

func (o harnessClaudeOAuth) RefreshTokenExpired(leeway time.Duration) bool {
	exp := o.RefreshTokenExpiry()
	if exp.IsZero() {
		return true
	}
	return time.Now().After(exp.Add(-leeway))
}

const claudeKeychainService = "Claude Code-credentials"

func loadClaudeCredentials(ctx context.Context) (*harnessClaudeCredentials, error) {
	cmd := exec.CommandContext(ctx, "security",
		"find-generic-password", "-s", claudeKeychainService, "-w")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("read keychain item %q: %w: %s",
			claudeKeychainService, err, bytes.TrimSpace(stderr.Bytes()))
	}

	var creds harnessClaudeCredentials
	if err := json.Unmarshal(stdout.Bytes(), &creds); err != nil {
		return nil, fmt.Errorf("parse keychain item %q: %w", claudeKeychainService, err)
	}
	if creds.OAuth.AccessToken == "" {
		return nil, fmt.Errorf("keychain item %q has no access token", claudeKeychainService)
	}
	return &creds, nil
}

func (c *harnessClaudeCredentials) Save(path string) (err error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	err = fs.WriteFile(path, data)
	if err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	err = fs.MakePrivate(path)
	if err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	return nil
}
