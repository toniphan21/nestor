package cli

import (
	"slices"
)

type Config struct {
	Dir              string            `yaml:"dir"`
	LogLevel         string            `yaml:"log_level"`
	PathReplacements map[string]string `yaml:"paths,omitempty"`
	Redacted         redacted          `yaml:"redacted,omitempty"`
	Hidden           *hidden           `yaml:"hidden,omitempty"`
}

func (c *Config) showPart(part string) bool {
	if c.Hidden == nil {
		return true
	}
	return !c.Hidden.isHidden(part, c.Hidden.Parts)
}

func (c *Config) showProfile(profile string) bool {
	if c.Hidden == nil {
		return true
	}
	return !c.Hidden.isHidden(profile, c.Hidden.Profiles)
}

func (c *Config) showHarness(harness string) bool {
	if c.Hidden == nil {
		return true
	}
	return !c.Hidden.isHidden(harness, c.Hidden.Harnesses)
}

func (c *Config) showSandboxSpec(spec string) bool {
	if c.Hidden == nil {
		return true
	}
	return !c.Hidden.isHidden(spec, c.Hidden.SandboxSpecs)
}

type redacted struct {
	Options  []string `yaml:"options"`
	Settings []string `yaml:"settings"`
	Envs     []string `yaml:"envs"`
}

type hidden struct {
	Parts        []string `yaml:"parts,omitempty"`
	Profiles     []string `yaml:"profiles,omitempty"`
	Harnesses    []string `yaml:"harnesses,omitempty"`
	SandboxSpecs []string `yaml:"sandbox_specs,omitempty"`
}

func (h *hidden) isHidden(name string, typ []string) bool {
	return slices.Index(typ, name) != -1
}

func DefaultConfig(wd string) *Config {
	secrets := []string{
		"CLAUDE_CODE_OAUTH_TOKEN",
		"ANTHROPIC_API_KEY",
		"proxy_authorization_bearer",
		"proxy_x_api_key",
		"proxy_x_goog_api_key",
	}
	pathReplacements := map[string]string{}
	if wd != "" {
		pathReplacements[wd] = "$PWD"
	}

	return &Config{
		Dir:              ".nestor",
		LogLevel:         "info",
		PathReplacements: pathReplacements,
		Redacted: redacted{
			Settings: secrets,
			Options:  secrets,
			Envs:     secrets,
		},
		Hidden: &hidden{
			Parts: []string{
				partTemplates,
			},
		},
	}
}

const (
	partDir         = "dir"
	partSpecPath    = "spec-path"
	partProfilePath = "profile-path"
	partTemplates   = "templates"
)
