package tui

import (
	"context"

	"nhatp.com/go/nestor"
)

type ConfigWorkspace struct {
	Path  string `yml:"path"`
	Name  string `yml:"name,omitempty"`
	Order int    `yml:"order,omitempty"`
}

type Config struct {
	Shell       string            `yml:"shell,omitempty"`
	DockerShell string            `yml:"docker_shell,omitempty"`
	Workspaces  []ConfigWorkspace `yml:"workspaces,omitempty"`
}

func (c *Config) Save(platform nestor.Platform) error {
	// TODO: marshall and save to platform.ConfigDir("tui.yml")
	return nil
}

type Workspace struct {
	*nestor.Workspace
	name string
}

func (w *Workspace) Name() string {
	if w.name == "" {
		return w.Workspace.Name()
	}
	return w.name
}

func loadWorkspaces(ctx context.Context, api nestor.API) []Workspace {
	// TODO:
	// - load nws := api.Workspaces(ctx)
	// - LoadConfig, build Workspace which embed nestor.Workspace, order by ConfigWorkspace.Order then Name
	// the Config store override name/order for nestor.Workspace only, matched via path, so if there is no matched
	// just use the nestor.Workspace as normal with their value.

	return nil
}

func LoadConfig(platform nestor.Platform) Config {
	// TODO: parse "tui.yml" in platform.ConfigDir("tui.yml"), if there is no file/error returns the default below
	return Config{
		Shell:       defaultShell(),
		DockerShell: "bash",
	}
}

func defaultShell() string {
	// TODO: detect common shell by this order: zsh/fish/bash/sh
	return "sh"
}
