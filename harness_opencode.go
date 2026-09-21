package nestor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nhatp.com/go/nestor/infra/fs"
)

func newHarnessOpenCode() Harness {
	return &harnessOpenCode{
		assets: embedAssets{
			"assets/opencode.dockerfile": {dst: "Dockerfile", file: true},
		},
	}
}

const OpenCodeConfigDir = "config-dir"
const OpenCodeSandboxConfigDirName = "config"
const OpenCodeSandboxDotLocalDirName = ".local"
const OpenCodeConfigFileName = "nestor.json"

const OpenCodeSettingProvider = "provider"
const OpenCodeSettingUpstream = "upstream"
const OpenCodeSettingProxyAuthorizationBearer = "proxy_authorization_bearer"
const OpenCodeSettingProxyXAPIKey = "proxy_x_api_key"
const OpenCodeSettingProxyXGoogAPIKey = "proxy_x_goog_api_key"

type harnessOpenCode struct {
	assets embedAssets
}

func (h *harnessOpenCode) Name() string {
	return string(HarnessOpenCode)
}

func (h *harnessOpenCode) DisplayName() string {
	return "OpenCode"
}

func (h *harnessOpenCode) defaultContainerHomeDir() string {
	return DefaultContainerHomeDir
}

func (h *harnessOpenCode) DefaultOptions(runtime Runtime) map[string]string {
	options := make(map[string]string)
	options[ProfileOptionDockerfile] = h.DefaultDockerfile(runtime)
	options[OpenCodeConfigDir] = runtime.Platform.HarnessDefaultOption(h.Name(), OpenCodeConfigDir)
	options[ProfileOptionContainerHomeDir] = h.defaultContainerHomeDir()
	return options
}

func (h *harnessOpenCode) DefaultDockerfile(runtime Runtime) string {
	return runtime.Platform.NestorDir("opencode", "Dockerfile")
}

func (h *harnessOpenCode) Init(runtime Runtime) error {
	log := runtime.Logger
	cp := filepath.Join(runtime.Platform.NestorDir(), "opencode")
	have, err := h.assets.save(cp)
	if err != nil {
		return err
	}

	if have {
		log.Info("opencode harness already exists, skip", slog.String("path", cp))
	} else {
		log.Info("saved builtin harness opencode", slog.String("path", cp))
	}
	return nil
}

func (h *harnessOpenCode) Syncers(sandbox Sandbox) []Syncer {
	profile := sandbox.Profile()
	s := &harnessOpenCodeSyncer{
		auth:      profile.Auth,
		configDir: profile.Options[OpenCodeConfigDir],
	}
	return []Syncer{s}
}

func (h *harnessOpenCode) Mounts(sandbox Sandbox) (map[string]SandboxMount, error) {
	profile := sandbox.Profile()
	ch := profile.Options[ProfileOptionContainerHomeDir]
	if ch == "" {
		ch = h.defaultContainerHomeDir()
	}

	cfp := sandbox.StateDir(OpenCodeSandboxConfigDirName)

	dlp := sandbox.StateDir(OpenCodeSandboxDotLocalDirName)
	if !fs.HasDir(dlp) {
		if err := os.MkdirAll(dlp, 0755); err != nil {
			return nil, err
		}
	}

	provider := profile.Settings[OpenCodeSettingProvider]
	config := map[string]any{
		"provider": map[string]any{
			provider: map[string]map[string]string{
				"options": {
					"baseURL": "{env:NESTOR_EXEC_PROXY_ADDR}",
					"apiKey":  "dummy",
				},
			},
		},
	}

	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, nil
	}
	configFilePath := sandbox.StateDir(OpenCodeConfigFileName)
	if err = fs.AtomicWriteFile(configFilePath, b, 0644); err != nil {
		return nil, nil
	}

	mounts := map[string]SandboxMount{
		cfp:            {Host: cfp, Target: filepath.Join(ch, ".config", "opencode")},
		dlp:            {Host: dlp, Target: filepath.Join(ch, ".local")},
		configFilePath: {Host: configFilePath, Target: filepath.Join(ch, OpenCodeConfigFileName), ReadOnly: true},
	}
	return mounts, nil
}

func (h *harnessOpenCode) StartEnv(sandbox Sandbox) map[string]string {
	profile := sandbox.Profile()
	ch := profile.Options[ProfileOptionContainerHomeDir]
	if ch == "" {
		ch = h.defaultContainerHomeDir()
	}

	return map[string]string{
		"OPENCODE_CONFIG": filepath.Join(ch, OpenCodeConfigFileName),
	}
}

func (h *harnessOpenCode) Exec(lease *Lease, req ExecRequest) HarnessExec {
	proxyAddr := lease.ProxyAddr()
	out := HarnessExec{
		Env: map[string]string{
			"NESTOR_EXEC_PROXY_ADDR": proxyAddr,
		},
		Command: []string{"opencode"},
	}

	if !req.Interactive {
		out.Command = append(out.Command, "run", "--format", "json")
	}
	out.Command = append(out.Command, "--dangerously-skip-permissions")

	profile := lease.Sandbox().Profile()
	if model := profile.Model(req.Model); model != "" {
		provider := profile.Settings[OpenCodeSettingProvider]
		if provider != "" {
			name := fmt.Sprintf("%s/%s", provider, model)
			out.Command = append(out.Command, "--model", name)
			out.Model = name
		}
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID != "" {
		out.Command = append(out.Command, "--session", sessionID)
		out.SessionID = sessionID
	}

	title := strings.TrimSpace(req.Title)
	if title != "" {
		out.Command = append(out.Command, "--title", shQuote(title))
	}

	if !req.Interactive && req.PromptFilePath != "" {
		out.Command = append(out.Command, "<", req.PromptFilePath)
	}
	return out
}

func (h *harnessOpenCode) ProxyRoute(lease *Lease) *ProxyRoute {
	profile := lease.Sandbox().Profile()
	if !profile.Proxy || profile.Auth == AuthCredentials {
		return nil
	}

	upstream, have := profile.Settings[OpenCodeSettingUpstream]
	if !have {
		return nil
	}

	ab, haveAB := profile.Settings[OpenCodeSettingProxyAuthorizationBearer]
	ak, haveAK := profile.Settings[OpenCodeSettingProxyXAPIKey]
	gk, haveGK := profile.Settings[OpenCodeSettingProxyXGoogAPIKey]
	if !haveAB && !haveAK && !haveGK {
		return nil
	}

	return &ProxyRoute{
		Target: upstream,
		Apply: func(h http.Header) {
			if haveAK {
				h.Set("x-api-key", ak)
			}

			if haveGK {
				h.Set("x-goog-api-key", gk)
			}

			if haveAB {
				h.Del("Authorization")
				h.Set("Authorization", "Bearer "+ab)
			}
		},
	}
}

func (h *harnessOpenCode) CaptureSessionID(line []byte) (string, bool) {
	var msg map[string]any
	if err := json.Unmarshal(line, &msg); err != nil {
		return "", false
	}
	id, ok := msg["sessionID"].(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

func (h *harnessOpenCode) ListSessions(lease *Lease) []HarnessSession {
	sandbox := lease.Sandbox()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := []string{"opencode", "session", "list", "--format", "json"}

	out := bytes.Buffer{}
	_, err := sandbox.Docker().Exec(ctx, lease.Sandbox().Container(), cmd, DockerExecOption{
		WorkDir: lease.WorkDir(),
		Stdout:  &out,
	})
	if err != nil {
		sandbox.Runtime().Logger.Warn("opencode list sessions error", slog.Any("error", err))
		return nil
	}

	result, err := parseOpenCodeSessions(out.Bytes())
	if err != nil {
		sandbox.Runtime().Logger.Warn("opencode parse sessions error", slog.Any("error", err))
		return nil
	}
	return result
}

var _ Harness = (*harnessOpenCode)(nil)

type harnessOpenCodeSyncer struct {
	auth      auth
	configDir string
}

func (h *harnessOpenCodeSyncer) Sync(ctx context.Context, sandbox Sandbox) error {
	targetDir := sandbox.StateDir()
	err := fs.MergeDir(h.configDir, filepath.Join(targetDir, OpenCodeSandboxConfigDirName))
	if err != nil {
		return err
	}
	return nil
}

type opencodeSession struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Created   int64  `json:"created"`
	ProjectID string `json:"projectId"`
	Directory string `json:"directory"`
}

func (s *opencodeSession) toHarnessSession() HarnessSession {
	return HarnessSession{
		ID:        s.ID,
		Title:     s.Title,
		ProjectID: s.ProjectID,
		Directory: s.Directory,
		CreatedAt: time.UnixMilli(s.Created).UTC(),
		UpdatedAt: time.UnixMilli(s.Updated).UTC(),
	}
}

func parseOpenCodeSessions(b []byte) ([]HarnessSession, error) {
	var raw []*opencodeSession
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse opencode sessions: %w", err)
	}

	out := make([]HarnessSession, len(raw))
	for i, s := range raw {
		out[i] = s.toHarnessSession()
	}
	return out, nil
}
