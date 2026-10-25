package nestor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"nhatp.com/go/nestor/infra/fs"
)

var _ ResolvedProfile = (*harnessClaudeProfile)(nil)

type harnessClaudeMode int

const (
	harnessClaudeModeSubscription harnessClaudeMode = iota
	harnessClaudeModeAPIKey
)

type harnessClaudeProfile struct {
	dockerfile       string
	claudeDir        string
	claudeConfigFile string
	containerHomeDir string
	mode             harnessClaudeMode
	secret           string
	profile          Profile
}

func (h *harnessClaudeProfile) Dockerfile() string {
	return h.dockerfile
}

func (h *harnessClaudeProfile) Model(request string) string {
	return h.profile.Model(request)
}

func (h *harnessClaudeProfile) SetupString() string {
	var mode, cost, privacy string
	switch h.mode {
	case harnessClaudeModeSubscription:
		mode = "Claude Subscription"
		cost = theme.Recommended("personal, cheap")
		privacy = theme.Warn("(OAuth token is set inside the sandbox; Anthropic is reported to ban accounts that use it as an API key)")
	case harnessClaudeModeAPIKey:
		mode = "Claude API Key"
		cost = theme.Aware("expensive")
		privacy = theme.Recommended("(key stays on the host; the sandbox never sees it)")
	}
	return fmt.Sprintf("%s - %s - %s %s", theme.Info(h.profile.Name), mode, cost, privacy)
}

func newHarnessClaude() Harness {
	return &harnessClaude{
		assets: embedAssets{
			"assets/claude.dockerfile": {dst: "Dockerfile", file: true},
		},
	}
}

const (
	ClaudeCodeOAuthTokenName = "CLAUDE_CODE_OAUTH_TOKEN"
	AnthropicAPIKeyName      = "ANTHROPIC_API_KEY"
	AnthropicAuthTokenName   = "ANTHROPIC_AUTH_TOKEN"
	AnthropicBaseURLName     = "ANTHROPIC_BASE_URL"
	ClaudeDir                = ".claude"
	ClaudeConfigFile         = ".claude.json"
	ClaudeCredentialsFile    = ".credentials.json"
)

type harnessClaude struct {
	assets embedAssets
}

func (h *harnessClaude) ID() harness {
	return HarnessClaudeCode
}

func (h *harnessClaude) Name() string {
	return string(HarnessClaudeCode)
}

func (h *harnessClaude) DisplayName() string {
	return "Claude Code"
}

func (h *harnessClaude) Init(runtime Runtime) error {
	log := runtime.Logger
	cp := runtime.Platform.ConfigDir("claude")
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

func (h *harnessClaude) ResolveProfile(runtime Runtime, profile Profile) (ResolvedProfile, error) {
	out := &harnessClaudeProfile{
		profile: profile,
	}

	out.dockerfile = profile.GetOption(ProfileOptionDockerfile, func() string {
		return runtime.Platform.ConfigDir("claude", "Dockerfile")
	})
	out.claudeDir = profile.GetOption(ClaudeDir, func() string {
		return path.Join(runtime.Platform.UserHomeDir(), ".claude")
	})
	out.claudeConfigFile = profile.GetOption(ClaudeConfigFile, func() string {
		return path.Join(runtime.Platform.UserHomeDir(), ".claude.json")
	})
	out.containerHomeDir = profile.GetOption(ProfileOptionContainerHomeDir, func() string {
		return DefaultContainerHomeDir
	})

	apiKey, haveAPIKey := profile.GetSetting(AnthropicAPIKeyName)
	oauthToken, haveOAuthToken := profile.GetSetting(ClaudeCodeOAuthTokenName)
	switch {
	case !haveAPIKey && !haveOAuthToken:
		return out, fmt.Errorf("%w: missing %q/%q in profile %s settings", ErrInvalid, AnthropicAPIKeyName, ClaudeCodeOAuthTokenName, profile.Name)

	case !haveAPIKey:
		out.secret = oauthToken
		out.mode = harnessClaudeModeSubscription

	case !haveOAuthToken:
		out.secret = apiKey
		out.mode = harnessClaudeModeAPIKey

	default:
		// oauth token out weight api key
		out.secret = oauthToken
		out.mode = harnessClaudeModeSubscription
	}
	return out, nil
}

func (h *harnessClaude) Syncers(sandbox Sandbox) []Syncer {
	profile, ok := sandbox.ResolvedProfile().(*harnessClaudeProfile)
	if !ok {
		return nil
	}
	return []Syncer{&harnessClaudeSyncer{profile: profile}}
}

func (h *harnessClaude) Mounts(sandbox Sandbox) (map[string]SandboxMount, error) {
	// .claude and .claude.json is copied from profile options to the sandbox.Dir() in Syncer
	profile, ok := sandbox.ResolvedProfile().(*harnessClaudeProfile)
	if !ok {
		return nil, nil
	}

	cd := sandbox.StateDir(ClaudeDir)
	cj := sandbox.StateDir(ClaudeConfigFile)
	mounts := map[string]SandboxMount{
		cd: {Host: cd, Target: filepath.Join(profile.containerHomeDir, ClaudeDir)},
		cj: {Host: cj, Target: filepath.Join(profile.containerHomeDir, ClaudeConfigFile)},
	}
	return mounts, nil
}

func (h *harnessClaude) StartEnv(sandbox Sandbox) map[string]string {
	profile, ok := sandbox.ResolvedProfile().(*harnessClaudeProfile)
	if !ok {
		return nil
	}

	env := make(map[string]string)
	switch profile.mode {
	case harnessClaudeModeSubscription:
		env[ClaudeCodeOAuthTokenName] = profile.secret
	case harnessClaudeModeAPIKey:
		// mode APIKey will use proxy, no need to set env
	}
	return env
}

func (h *harnessClaude) Exec(lease *Lease, req ExecRequest) (HarnessExec, error) {
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

	profile, ok := lease.Sandbox().ResolvedProfile().(*harnessClaudeProfile)
	if !ok {
		return out, fmt.Errorf("%w: resolved profile is not *harnessClaudeProfile", ErrInvalid)
	}

	switch profile.mode {
	case harnessClaudeModeAPIKey:
		ap := lease.AuthProxyAddr()

		out.Env[EnvVarAuthProxyAddress] = ap
		out.Env[AnthropicBaseURLName] = ap
		out.Env[AnthropicAuthTokenName] = "dummy"
	}

	if mcpProxyURL, have := lease.MCPProxyURL(); have {
		out.Env[EnvVarProxyAddress] = lease.ProxyAddr()

		config := map[string]any{}
		config["mcpServers"] = map[string]any{
			BinaryName: map[string]any{
				"type": "http",
				"url":  mcpProxyURL,
			},
		}

		if b, err := json.MarshalIndent(config, "", "  "); err == nil {
			if fp, err := lease.StageFile("mcp.json", b, 0o644); err == nil {
				out.Command = append(out.Command, "--mcp-config", fp)
			}
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
	return out, nil
}

func (h *harnessClaude) ProxyRoute(lease *Lease) *ProxyRoute {
	profile, ok := lease.Sandbox().ResolvedProfile().(*harnessClaudeProfile)
	if !ok {
		return nil
	}

	if profile.mode != harnessClaudeModeAPIKey {
		return nil
	}

	return &ProxyRoute{
		Target: "https://api.anthropic.com",
		Apply: func(h http.Header) {
			h.Set("x-api-key", profile.secret)
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

func (h *harnessClaude) ListSessions(_ context.Context, path string, sandboxes ...Sandbox) []HarnessSession {
	var out []HarnessSession
	for _, sandbox := range sandboxes {
		cd := sandbox.StateDir(ClaudeDir)
		wd, _, ok := sandbox.ResolveWorkDir(path)
		if !ok {
			continue
		}

		for _, s := range readSessionFiles(cd) {
			if s.CWD != wd {
				continue
			}
			out = append(out, s.toHarnessSession())
		}
	}
	return out
}

var _ Harness = (*harnessClaude)(nil)

type harnessClaudeSyncer struct {
	profile *harnessClaudeProfile
}

func (h *harnessClaudeSyncer) Sync(ctx context.Context, sandbox Sandbox) error {
	targetDir := sandbox.StateDir()
	var err error
	err = fs.MergeDir(h.profile.claudeDir, filepath.Join(targetDir, ClaudeDir))
	if err != nil {
		return err
	}

	err = fs.CopyFile(h.profile.claudeConfigFile, filepath.Join(targetDir, ClaudeConfigFile))
	if err != nil {
		return err
	}
	return nil
}

var _ Syncer = (*harnessClaudeSyncer)(nil)

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
	defer func() { _ = fh.Close() }()

	var timestamp *time.Time
	hasCustomTitle := false

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
