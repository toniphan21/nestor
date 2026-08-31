package nestor

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

const EnvVarNestorDir = "NESTOR_DIR"

type OSKind string

const OSMacOS OSKind = "macos"
const OSLinux OSKind = "linux"
const OSWindows OSKind = "windows"

type Platform interface {
	UserHomeDir() string

	NestorDir(path ...string) string

	SandboxDir(path ...string) string

	ProfileYmlFile() string

	SandboxYmlFile() string

	OS() OSKind

	HarnessDefaultOption(harness string, name string) string

	Clone(dir string) Platform
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

	dir := os.Getenv(EnvVarNestorDir)
	if dir == "" {
		dir = filepath.Join(home, ".nestor")
	}
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("nestor: %s must be absolute, got %q", EnvVarNestorDir, dir)
	}
	return &platform{os: kind, home: home, nestorDir: dir}, nil
}

type platform struct {
	os        OSKind
	home      string
	nestorDir string
}

func (p *platform) UserHomeDir() string {
	return p.home
}

func (p *platform) NestorDir(path ...string) string {
	if len(path) == 0 {
		return p.nestorDir
	}

	args := []string{p.nestorDir}
	args = append(args, path...)
	return filepath.Join(args...)
}

func (p *platform) SandboxDir(path ...string) string {
	sbd := filepath.Join(p.nestorDir, "sandbox")
	if len(path) == 0 {
		return sbd
	}

	args := []string{sbd}
	args = append(args, path...)
	return filepath.Join(args...)
}

func (p *platform) ProfileYmlFile() string {
	return p.NestorDir("profile.yml")
}

func (p *platform) SandboxYmlFile() string {
	return p.NestorDir("sandbox.yml")
}

func (p *platform) HarnessDefaultOption(harness string, name string) string {
	if p.OS() == OSWindows {
		panic(fmt.Sprintf("nestor: unsupported platform %q - please write your own Platform", p.OS()))
	}

	switch harness {
	case "claude":
		switch name {
		case ".claude":
			return path.Join(p.UserHomeDir(), ".claude")
		case ".claude.json":
			return path.Join(p.UserHomeDir(), ".claude.json")
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

func (p *platform) Clone(dir string) Platform {
	return &platform{os: p.os, home: p.home, nestorDir: dir}
}

var _ Platform = (*platform)(nil)
