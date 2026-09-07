package nestor

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

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
const OpenCodeSandboxConfigDirName = "config-opencode"
const OpenCodeSandboxDotLocalDirName = ".local"

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
	return "/home/agent"
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

	cfp := sandbox.Dir(OpenCodeSandboxConfigDirName)

	dlp := sandbox.Dir(OpenCodeSandboxDotLocalDirName)
	if !fs.HasDir(dlp) {
		if err := os.MkdirAll(dlp, 0755); err != nil {
			return nil, err
		}
	}

	mounts := map[string]SandboxMount{
		cfp: {Host: cfp, Target: filepath.Join(ch, ".config", "opencode")},
		dlp: {Host: dlp, Target: filepath.Join(ch, ".local")},
	}
	return mounts, nil
}

func (h *harnessOpenCode) StartEnv(sandbox Sandbox) map[string]string {
	return nil
}

func (h *harnessOpenCode) ExecEnv(lease *Lease, req ExecRequest) map[string]string {
	return nil
}

func (h *harnessOpenCode) ExecCommand(lease *Lease, req ExecRequest) []string {
	cmd := []string{
		"opencode",
		"run",
		"--dangerously-skip-permissions",
		"--format", "json",
	}

	//profile := lease.Sandbox().Profile()
	//if model := profile.Model(req.Model); model != "" {
	//	provider := profile.Options[OpenCodeSettingProvider]
	//	name := fmt.Sprintf("%s/%s", provider, model)
	//	cmd = append(cmd, "--model", name)
	//}

	cmd = append(cmd, "<", req.PromptFilePath)
	return cmd
}

func (h *harnessOpenCode) ProxyRoute(lease *Lease, req ExecRequest) *ProxyRoute {
	return nil
}

var _ Harness = (*harnessOpenCode)(nil)

type harnessOpenCodeSyncer struct {
	auth      auth
	configDir string
}

func (h *harnessOpenCodeSyncer) Sync(ctx context.Context, sandbox Sandbox) error {
	targetDir := sandbox.Dir()
	err := fs.MergeDir(h.configDir, filepath.Join(targetDir, OpenCodeSandboxConfigDirName))
	if err != nil {
		return err
	}
	return nil
}
