package nestor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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

const ClaudeCodeOAuthTokenName = "CLAUDE_CODE_OAUTH_TOKEN"
const AnthropicAPIKeyName = "ANTHROPIC_API_KEY"
const AnthropicAuthTokenName = "ANTHROPIC_AUTH_TOKEN"
const AnthropicBaseURLName = "ANTHROPIC_BASE_URL"
const ClaudeDir = ".claude"
const ClaudeConfigFile = ".claude.json"
const ClaudeCredentialsFile = ".credentials.json"

type harnessClaude struct {
	assets embedAssets
}

func (h *harnessClaude) Name() string {
	return string(HarnessClaudeCode)
}

func (h *harnessClaude) DisplayName() string {
	return "Claude Code"
}

func (h *harnessClaude) defaultContainerHomeDir() string {
	return "/home/agent"
}

func (h *harnessClaude) DefaultDockerfile(runtime Runtime) string {
	return runtime.Platform.NestorDir("claude", "Dockerfile")
}

func (h *harnessClaude) DefaultOptions(runtime Runtime) map[string]string {
	options := make(map[string]string)
	options[ProfileOptionDockerfile] = h.DefaultDockerfile(runtime)
	options[ClaudeDir] = runtime.Platform.HarnessDefaultOption(h.Name(), ClaudeDir)
	options[ClaudeConfigFile] = runtime.Platform.HarnessDefaultOption(h.Name(), ClaudeConfigFile)
	options[ProfileOptionContainerHomeDir] = h.defaultContainerHomeDir()
	return options
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

func (h *harnessClaude) Syncers(sandbox Sandbox) []Syncer {
	runtime, profile := sandbox.Runtime(), sandbox.Profile()
	s := &harnessClaudeSyncer{
		auth:       profile.Auth,
		claudeDir:  profile.Options[ClaudeDir],
		claudeJson: profile.Options[ClaudeConfigFile],
		os:         runtime.Platform.OS(),
	}
	return []Syncer{s}
}

func (h *harnessClaude) Mounts(sandbox Sandbox) (map[string]SandboxMount, error) {
	// .claude and .claude.json is copied from profile options to the sandbox.Dir() in Syncer
	profile := sandbox.Profile()
	ch := profile.Options[ProfileOptionContainerHomeDir]
	if ch == "" {
		ch = h.defaultContainerHomeDir()
	}

	cd := sandbox.Dir(ClaudeDir)
	cj := sandbox.Dir(ClaudeConfigFile)
	mounts := map[string]SandboxMount{
		cd: {Host: cd, Target: filepath.Join(ch, ClaudeDir)},
		cj: {Host: cj, Target: filepath.Join(ch, ClaudeConfigFile)},
	}
	return mounts, nil
}

func (h *harnessClaude) StartEnv(sandbox Sandbox) map[string]string {
	var env = make(map[string]string)
	profile := sandbox.Profile()
	switch profile.Auth {
	case AuthCredentials:
		if tok, ok := profile.Settings[ClaudeCodeOAuthTokenName]; ok {
			env[ClaudeCodeOAuthTokenName] = tok
		}
	case AuthAPIKey:
		if !profile.Proxy {
			if apiKey, ok := profile.Settings[AnthropicAPIKeyName]; ok {
				env[AnthropicAPIKeyName] = apiKey
			}
		}
	}
	return env
}

func (h *harnessClaude) Exec(lease *Lease, req ExecRequest) HarnessExec {
	out := HarnessExec{
		Env: make(map[string]string),
		Command: []string{
			"claude",
			"--dangerously-skip-permissions",
			"--output-format stream-json",
			"--verbose",
		},
	}

	profile := lease.Sandbox().Profile()
	switch profile.Auth {
	case AuthAPIKey:
		if profile.Proxy {
			out.Env[AnthropicBaseURLName] = lease.ProxyAddr()
			out.Env[AnthropicAuthTokenName] = "dummy"
		}
	}

	if model := profile.Model(req.Model); model != "" {
		out.Command = append(out.Command, "--model", model)
		out.Model = model
	}

	if req.SessionID != "" {
		out.Command = append(out.Command, "--resume", req.SessionID)
		out.SessionID = req.SessionID
	}

	out.Command = append(out.Command, "--print")
	out.Command = append(out.Command, "<", req.PromptFilePath)

	return out
}

func (h *harnessClaude) ProxyRoute(lease *Lease, req ExecRequest) *ProxyRoute {
	profile := lease.Sandbox().Profile()
	if !profile.Proxy || profile.Auth == AuthCredentials {
		return nil
	}

	apiKey, have := profile.Settings[AnthropicAPIKeyName]
	if !have {
		return nil
	}

	return &ProxyRoute{
		Target: "https://api.anthropic.com",
		Apply: func(h http.Header) {
			h.Set("x-api-key", apiKey)
			h.Del("Authorization")
		},
	}
}

func (h *harnessClaude) CaptureSessionID(line []byte) (string, bool) {
	var msg map[string]any
	if err := json.Unmarshal(line, &msg); err != nil {
		return "", false
	}
	id, ok := msg["session_id"].(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
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
	err = fs.MergeDir(h.claudeDir, filepath.Join(targetDir, ClaudeDir))
	if err != nil {
		return err
	}

	err = fs.CopyFile(h.claudeJson, filepath.Join(targetDir, ClaudeConfigFile))
	if err != nil {
		return err
	}

	switch h.auth {
	case AuthCredentials:
		_, have := sandbox.Profile().Settings[ClaudeCodeOAuthTokenName]
		if !have && h.os == OSMacOS {
			return h.saveCredentialsFromSecurity(ctx, targetDir)
		}
	}
	return nil
}

func (h *harnessClaudeSyncer) saveCredentialsFromSecurity(ctx context.Context, targetDir string) error {
	cc, err := loadClaudeCredentials(ctx)
	if err != nil {
		return err
	}
	if cc.OAuth.RefreshTokenExpired(1 * time.Hour) {
		return errors.New("refresh token expired")
	}

	dst := filepath.Join(targetDir, ClaudeDir, ClaudeCredentialsFile)
	return cc.Save(dst)
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
