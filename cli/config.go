package cli

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
)

type ConfigWorkspace struct {
	Path  string `yaml:"path"`
	Name  string `yaml:"name,omitempty"`
	Order int    `yaml:"order,omitempty"`
}

type Config struct {
	Shell       string            `yaml:"shell,omitempty"`
	DockerShell string            `yaml:"docker_shell,omitempty"`
	Workspaces  []ConfigWorkspace `yaml:"workspaces,omitempty"`
}

func (c *Config) Save(platform nestor.Platform) error {
	fp := platform.ConfigDir("tui.yml")
	if err := fs.MkdirAll(filepath.Dir(fp)); err != nil {
		return err
	}

	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return fs.AtomicWriteFile(fp, b, 0o644)
}

type Workspace struct {
	*nestor.Workspace
	name  string
	order int
}

func (w *Workspace) Name() string {
	if w.name == "" {
		return w.Workspace.Name()
	}
	return w.name
}

func Workspaces(ctx context.Context, api nestor.API) []Workspace {
	nws := api.Workspaces(ctx)
	platform := api.Runtime().Platform
	cfg := LoadConfig(platform)

	if !fs.HasFile(platform.ConfigDir("tui.yml")) {
		cfg.Workspaces = defaultConfigWorkspaces(nws)
		_ = cfg.Save(platform)
	}

	byPath := make(map[string]ConfigWorkspace, len(cfg.Workspaces))
	for _, cw := range cfg.Workspaces {
		byPath[cw.Path] = cw
	}

	// nestor.Workspace is the source of truth: a ConfigWorkspace only ever
	// overrides a workspace that still exists. One with no matching
	// nestor.Workspace is left untouched in cfg.Workspaces (and thus in
	// tui.yml) rather than pruned, in case the workspace reappears later.
	workspaces := make([]Workspace, len(nws))
	for i := range nws {
		w := Workspace{Workspace: &nws[i]}
		if cw, ok := byPath[nws[i].Path()]; ok {
			w.name = cw.Name
			w.order = cw.Order
		}
		workspaces[i] = w
	}

	slices.SortFunc(workspaces, func(a, b Workspace) int {
		if a.order != b.order {
			return a.order - b.order
		}
		return strings.Compare(a.Name(), b.Name())
	})

	return workspaces
}

// defaultConfigWorkspaces snapshots the current nestor workspaces, in their
// default display order, so a freshly created tui.yml starts pre-populated
// and ready to edit instead of empty.
func defaultConfigWorkspaces(nws []nestor.Workspace) []ConfigWorkspace {
	sorted := make([]nestor.Workspace, len(nws))
	copy(sorted, nws)
	slices.SortFunc(sorted, func(a, b nestor.Workspace) int {
		return strings.Compare(a.Name(), b.Name())
	})

	cws := make([]ConfigWorkspace, len(sorted))
	for i, w := range sorted {
		cws[i] = ConfigWorkspace{Path: w.Path(), Name: w.Name(), Order: i}
	}
	return cws
}

func LoadConfig(platform nestor.Platform) Config {
	def := Config{
		Shell:       defaultShell(),
		DockerShell: "bash",
	}

	fp := platform.ConfigDir("tui.yml")
	if !fs.HasFile(fp) {
		return def
	}

	b, err := fs.AtomicReadFile(fp)
	if err != nil {
		return def
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return def
	}
	return cfg
}

func defaultShell() string {
	for _, sh := range []string{"zsh", "fish", "bash", "sh"} {
		if _, err := exec.LookPath(sh); err == nil {
			return sh
		}
	}
	return "sh"
}
