package nestor

import (
	"context"
	"slices"
)

type Workspace struct {
	path      string
	name      string
	mounts    []WorkspaceMount
	sandboxes []WorkspaceSandbox
}

func (w Workspace) Path() string                  { return w.path }
func (w Workspace) Name() string                  { return w.name }
func (w Workspace) Mounts() []WorkspaceMount      { return slices.Clone(w.mounts) }
func (w Workspace) Sandboxes() []WorkspaceSandbox { return slices.Clone(w.sandboxes) }

type WorkspaceMount struct {
	spec     SandboxSpec
	typ      MountType
	readOnly bool
	target   string
}

func (m WorkspaceMount) Spec() SandboxSpec { return m.spec }
func (m WorkspaceMount) Type() MountType   { return m.typ }
func (m WorkspaceMount) ReadOnly() bool    { return m.readOnly }
func (m WorkspaceMount) Target() string    { return m.target }

type WorkspaceSandbox struct {
	mount       WorkspaceMount
	sandbox     Sandbox
	workDir     string
	hostWorkDir string
	sessions    []WorkspaceSession
}

func (s WorkspaceSandbox) Mount() WorkspaceMount        { return s.mount }
func (s WorkspaceSandbox) Sandbox() Sandbox             { return s.sandbox }
func (s WorkspaceSandbox) WorkDir() string              { return s.workDir }
func (s WorkspaceSandbox) HostWorkDir() string          { return s.hostWorkDir }
func (s WorkspaceSandbox) Sessions() []WorkspaceSession { return slices.Clone(s.sessions) }

type WorkspaceSession struct {
	id    string
	title string
}

func (s WorkspaceSession) ID() string    { return s.id }
func (s WorkspaceSession) Title() string { return s.title }

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

				ws.mounts = append(ws.mounts, WorkspaceMount{spec: spec, typ: m.Type, readOnly: m.ReadOnly, target: m.At})
				ws.sandboxes = append(ws.sandboxes, collectWorkspaceSandboxes(ctx, runtime, spec, m, path)...)
			}
		}
		result = append(result, ws)
	}

	return result
}

func collectWorkspaceSandboxes(ctx context.Context, runtime Runtime, spec SandboxSpec, specM SandboxSpecMount, path string) []WorkspaceSandbox {
	sandboxes, err := spec.ListSandboxes(runtime)
	if err != nil {
		return nil
	}

	var out []WorkspaceSandbox
	for _, sandbox := range sandboxes {
		wd, hwd, ok := sandbox.ResolveWorkDir(path)
		if !ok {
			continue
		}

		sb := WorkspaceSandbox{
			mount:       WorkspaceMount{spec: spec, typ: specM.Type, readOnly: specM.ReadOnly, target: specM.At},
			sandbox:     sandbox,
			workDir:     wd,
			hostWorkDir: hwd,
			sessions:    collectWorkspaceSessions(ctx, path, sandbox),
		}
		out = append(out, sb)
	}
	return out
}

func collectWorkspaceSessions(ctx context.Context, path string, sb Sandbox) []WorkspaceSession {
	var out []WorkspaceSession
	ss := sb.Harness().ListSessions(ctx, path, sb)
	for _, v := range ss {
		out = append(out, WorkspaceSession{id: v.ID, title: v.Title})
	}
	return out
}
