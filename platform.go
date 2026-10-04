package nestor

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const EnvVarNestorDir = "NESTOR_DIR"

type OSKind string

const (
	OSMacOS   OSKind = "macos"
	OSLinux   OSKind = "linux"
	OSWindows OSKind = "windows"
)

type Platform interface {
	UserHomeDir() string

	ConfigDir(elem ...string) string

	DataDir(elem ...string) string

	StateDir(elem ...string) string

	SandboxDir(elem ...string) string

	ShareSandboxSpecDir(elem ...string) string

	ProfileYmlFile() string

	SandboxYmlFile() string

	MCPYmlFile() string

	SandboxSlugsFile() string

	OS() OSKind

	HarnessDefaultOption(harness string, name string) string

	Clone(configDir, dataDir, stateDir string) Platform
}

func DefaultPlatform() (Platform, error) {
	switch runtime.GOOS {
	case "darwin":
		return newUnixPlatform(OSMacOS)
	case "linux":
		return newUnixPlatform(OSLinux)
	default:
		return nil, fmt.Errorf("%w: OS %s", ErrNotSupported, runtime.GOOS)
	}
}

func newUnixPlatform(kind OSKind) (Platform, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("nestor: resolve home dir: %w", err)
	}

	xdgDir := func(name string, defaultValue string) string {
		dir := strings.TrimSpace(os.Getenv(name))
		if dir == "" {
			return defaultValue
		}
		return dir
	}

	return &platform{
		os:        kind,
		home:      home,
		configDir: xdgDir("XDG_CONFIG_HOME", filepath.Join(home, ".config", "nestor")),
		dataDir:   xdgDir("XDG_DATA_HOME", filepath.Join(home, ".local", "share", "nestor")),
		stateDir:  xdgDir("XDG_STATE_HOME", filepath.Join(home, ".local", "state", "nestor")),
	}, nil
}

type platform struct {
	os        OSKind
	home      string
	configDir string
	dataDir   string
	stateDir  string
}

func (p *platform) UserHomeDir() string {
	return p.home
}

func (p *platform) ConfigDir(elem ...string) string {
	return filepath.Join(append([]string{p.configDir}, elem...)...)
}

func (p *platform) DataDir(elem ...string) string {
	return filepath.Join(append([]string{p.dataDir}, elem...)...)
}

func (p *platform) StateDir(elem ...string) string {
	return filepath.Join(append([]string{p.stateDir}, elem...)...)
}

func (p *platform) SandboxDir(elem ...string) string {
	return p.StateDir(append([]string{"sandbox"}, elem...)...)
}

func (p *platform) ShareSandboxSpecDir(elem ...string) string {
	return p.DataDir(append([]string{"sandbox-spec"}, elem...)...)
}

func (p *platform) ProfileYmlFile() string {
	return p.ConfigDir("profile.yml")
}

func (p *platform) MCPYmlFile() string {
	return p.ConfigDir("mcp.yml")
}

func (p *platform) SandboxYmlFile() string {
	return p.ConfigDir("sandbox.yml")
}

func (p *platform) SandboxSlugsFile() string {
	return p.ConfigDir("sandbox-slugs.txt")
}

func (p *platform) HarnessDefaultOption(harness string, name string) string {
	if p.OS() == OSWindows {
		panic(fmt.Sprintf("nestor: unsupported platform %q - please write your own Platform", p.OS()))
	}

	switch harness {
	case string(HarnessClaudeCode):
		switch name {
		case ClaudeDir:
			return path.Join(p.UserHomeDir(), ".claude")
		case ClaudeConfigFile:
			return path.Join(p.UserHomeDir(), ".claude.json")
		default:
			return ""
		}

	case string(HarnessOpenCode):
		switch name {
		case OpenCodeConfigDir:
			return path.Join(p.UserHomeDir(), ".config", "opencode")
		default:
			return ""
		}

	default:
		return ""
	}
}

func (p *platform) OS() OSKind {
	return p.os
}

func (p *platform) Clone(configDir, dataDir, stateDir string) Platform {
	return &platform{os: p.os, home: p.home, configDir: configDir, dataDir: dataDir, stateDir: stateDir}
}

var _ Platform = (*platform)(nil)
