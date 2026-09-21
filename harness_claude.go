package nestor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	return DefaultContainerHomeDir
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

	cd := sandbox.StateDir(ClaudeDir)
	cj := sandbox.StateDir(ClaudeConfigFile)
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
		},
	}

	if !req.Interactive {
		out.Command = append(out.Command, "--output-format", "stream-json")
		out.Command = append(out.Command, "--verbose")
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

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID != "" {
		out.Command = append(out.Command, "--resume", sessionID)
		out.SessionID = sessionID
	}

	title := strings.TrimSpace(req.Title)
	if title != "" {
		out.Command = append(out.Command, "--name", shQuote(title))
	}

	if req.InstructionFiles.HasFiles() {
		out.Command = append(out.Command, "--append-system-prompt-file", req.InstructionFiles.CombinedPath)
	}

	if !req.Interactive {
		out.Command = append(out.Command, "--print")

		if req.PromptFilePath != "" {
			out.Command = append(out.Command, "<", req.PromptFilePath)
		}
	}
	return out
}

func (h *harnessClaude) ProxyRoute(lease *Lease) *ProxyRoute {
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

func (h *harnessClaude) ListSessions(lease *Lease) []HarnessSession {
	sandbox := lease.Sandbox()
	cd := sandbox.StateDir(ClaudeDir)
	wd := lease.WorkDir()

	var out []HarnessSession
	for _, s := range readSessionFiles(cd) {
		if s.CWD != wd {
			continue
		}
		out = append(out, s.toHarnessSession())
	}
	return out
}

var _ Harness = (*harnessClaude)(nil)

type harnessClaudeSyncer struct {
	auth       auth
	claudeDir  string
	claudeJson string
	os         OSKind
}

func (h *harnessClaudeSyncer) Sync(ctx context.Context, sandbox Sandbox) error {
	targetDir := sandbox.StateDir()
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

type claudeSession struct {
	Dir            string // project dir name (slug, opaque)
	Path           string // abs path to the .jsonl
	SessionID      string // file stem
	CWD            string // from first event that carries it
	Title          string // ai-title or custom-title - last one win
	FirstTimestamp time.Time
	LastTimestamp  time.Time
}

func readSessionFiles(path string) []*claudeSession {
	projects := filepath.Join(path, "projects")

	dirs, err := os.ReadDir(projects)
	if err != nil {
		return nil
	}

	var out []*claudeSession
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		dir := filepath.Join(projects, d.Name())

		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}

			s, err := readSessionFile(dir, filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}

			out = append(out, s)
		}
	}
	return out
}

func readSessionFile(dir, file string) (*claudeSession, error) {
	s := &claudeSession{
		Dir:       filepath.Base(dir),
		Path:      file,
		SessionID: strings.TrimSuffix(filepath.Base(file), ".jsonl"),
	}

	fh, err := os.Open(file)
	if err != nil {
		return s, err
	}
	defer fh.Close()

	var timestamp *time.Time
	var hasCustomTitle = false

	err = scanLines(fh, func(line []byte) {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			return
		}

		// collect First and Last timestamp
		if t, have := event["timestamp"]; have {
			if v, ok := t.(string); ok {
				if tt, err := time.Parse(time.RFC3339, v); err == nil {
					if timestamp == nil {
						timestamp = &tt
						s.FirstTimestamp = tt
					} else {
						if tt.After(*timestamp) {
							timestamp = &tt
						}
					}
				}
			}
		}

		// collect title
		if t, have := event["type"]; have {
			if v, ok := t.(string); ok {
				switch v {
				case "ai-title":
					if !hasCustomTitle {
						if tt, hv := event["aiTitle"]; hv {
							if vv, ok := tt.(string); ok {
								s.Title = vv
							}
						}
					}

				case "custom-title":
					if tt, hv := event["customTitle"]; hv {
						if vv, ok := tt.(string); ok {
							s.Title = vv
							hasCustomTitle = true
						}
					}

				}
			}
		}

		// collect cwd
		if t, have := event["cwd"]; have {
			if v, ok := t.(string); ok {
				s.CWD = v
			}
		}
	})

	s.LastTimestamp = *timestamp

	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *claudeSession) toHarnessSession() HarnessSession {
	return HarnessSession{
		ID:        s.SessionID,
		ProjectID: s.Dir,
		Title:     s.Title,
		Directory: s.CWD,
		CreatedAt: s.FirstTimestamp,
		UpdatedAt: s.LastTimestamp,
	}
}
