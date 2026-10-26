package nestor

import (
	"context"
	"slices"
)

type Workspace struct {
	path   string
	name   string
	mounts []WorkspaceMount
}

func (w Workspace) Path() string             { return w.path }
func (w Workspace) Name() string             { return w.name }
func (w Workspace) Mounts() []WorkspaceMount { return slices.Clone(w.mounts) }

type WorkspaceMount struct {
	spec      SandboxSpec
	typ       MountType
	readOnly  bool
	target    string
	sandboxes []WorkspaceSandbox
}

func (m WorkspaceMount) Spec() SandboxSpec             { return m.spec }
func (m WorkspaceMount) Type() MountType               { return m.typ }
func (m WorkspaceMount) ReadOnly() bool                { return m.readOnly }
func (m WorkspaceMount) Target() string                { return m.target }
func (m WorkspaceMount) Sandboxes() []WorkspaceSandbox { return slices.Clone(m.sandboxes) }

type WorkspaceSandbox struct {
	sandbox     Sandbox
	workDir     string
	hostWorkDir string
}

func (s WorkspaceSandbox) Sandbox() Sandbox    { return s.sandbox }
func (s WorkspaceSandbox) WorkDir() string     { return s.workDir }
func (s WorkspaceSandbox) HostWorkDir() string { return s.hostWorkDir }

func collectWorkspaces(ctx context.Context, api API) []Workspace {
	var result []Workspace

	paths := make(map[string][]SandboxSpec)
	specs := api.Runtime().Registry.SandboxSpecs()
	for _, v := range specs {
		for _, m := range v.Mounts {
			paths[m.Path] = append(paths[m.Path], v)
		}
	}

	runtime := api.Runtime()
	for path, specs := range paths {
		ws := Workspace{
			path: path,
			name: runtime.Template.MakeWorkspaceName(path),
		}

		for _, spec := range specs {
			for _, m := range spec.Mounts {
				if m.Path != path {
					continue
				}

				ws.mounts = append(ws.mounts, WorkspaceMount{
					spec:      spec,
					typ:       m.Type,
					readOnly:  m.ReadOnly,
					target:    m.At,
					sandboxes: collectWorkspaceSandboxes(ctx, runtime, spec, path),
				})
			}
		}
		result = append(result, ws)
	}

	return result
}

func collectWorkspaceSandboxes(ctx context.Context, runtime Runtime, spec SandboxSpec, path string) []WorkspaceSandbox {
	sandboxes, err := spec.ListSandboxes(ctx, runtime)
	if err != nil {
		return nil
	}

	var out []WorkspaceSandbox
	for _, sandbox := range sandboxes {
		wd, hwd, ok := sandbox.ResolveWorkDir(path)
		if !ok {
			continue
		}

		out = append(out, WorkspaceSandbox{
			sandbox:     sandbox,
			workDir:     wd,
			hostWorkDir: hwd,
		})
	}
	return out
}
