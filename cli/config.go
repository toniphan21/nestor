package cli

import (
	"slices"
)

type Config struct {
	Dir               string            `yaml:"dir"`
	Root              string            `yaml:"root"`
	LogLevel          string            `yaml:"log_level"`
	PathReplacements  map[string]string `yaml:"paths,omitempty"`
	Redacted          redacted          `yaml:"redacted,omitempty"`
	PowerOverhead     float64           `yaml:"power_overhead"`
	SectorSize        uint64            `yaml:"sector_size"`
	TotalBytesWritten totalBytesWritten `yaml:"total_bytes_written"`
	Hidden            *hidden           `yaml:"hidden,omitempty"`
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

type totalBytesWritten struct {
	Overhead   uint64            `yaml:"overhead"`
	Format     string            `yaml:"format"`
	Thresholds map[uint64]string `yaml:"thresholds"`
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

const defaultTotalBytesWrittenFormat = "TBW=%.3f"

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
		Root:             "",
		LogLevel:         "info",
		PowerOverhead:    1,
		SectorSize:       512,
		PathReplacements: pathReplacements,
		Redacted: redacted{
			Settings: secrets,
			Options:  secrets,
			Envs:     secrets,
		},
		Hidden: &hidden{
			Parts: []string{
				partTime,
				partSystem,
				partOS,
				partTemplates,
			},
		},
		TotalBytesWritten: totalBytesWritten{
			Overhead: 0,
			Format:   defaultTotalBytesWrittenFormat,
			Thresholds: map[uint64]string{
				1000:  "gray",   // 0-1000 GB      - for 32G: ~1.5% wear - normal
				5000:  "none",   // 1000-5000 GB   - for 32G: ~8%–15% wear - normal
				10000: "yellow", // 5000-10000 GB  - for 32G: ~15%–30% wear - normal | 16GB: a bit worn
				15000: "red",    // 10000-15000 GB - for 32G: ~30% wear - normal     | 16G: think about replacement
			},
		},
	}
}

const (
	partTime        = "time"
	partSystem      = "system"
	partDir         = "dir"
	partSpecPath    = "spec-path"
	partProfilePath = "profile-path"
	partOS          = "os"
	partTemplates   = "templates"
)
