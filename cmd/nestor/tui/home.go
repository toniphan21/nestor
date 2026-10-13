package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func sessionTitle(s nestor.HarnessSession) string {
	if s.Title != "" {
		return s.Title
	}
	return "last opened: " + s.UpdatedAt.Local().Format("2006-01-02 15:04")
}

type homeTickMsg struct{ gen int }

type homeDataMsg struct {
	gen        int
	workspaces []homeWorkspace
	uhome      string
	err        error
}

type homeSessionsMsg struct {
	gen      int
	sandbox  *homeSandbox
	sessions []nestor.HarnessSession
}

type homeSandbox struct {
	wsPath      string
	id          string
	mountType   nestor.MountType
	sandbox     nestor.Sandbox
	workDir     string
	hostWorkDir string
	isFree      bool
}

func (v *homeSandbox) statusDisplay() string {
	if v.isFree {
		return pterm.Green("free")
	}
	return pterm.Red("busy")
}

func (v *homeSandbox) extraDisplay(uhome string) string {
	extra := cutHome(v.hostWorkDir, uhome)
	if v.workDir != v.hostWorkDir {
		extra += " → " + cutHome(v.workDir, uhome)
	}
	return extra
}

func (v *homeSandbox) useSessionsContent(sessions []nestor.HarnessSession) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# sessions for sandbox %q\n\n", v.id)
	if len(sessions) == 0 {
		sb.WriteString("# no sessions found\n")
		return sb.String()
	}
	for _, s := range sessions {
		fmt.Fprintf(&sb, "- %s: %s\n", s.ID, sessionTitle(s))
	}
	return sb.String()
}

func (v *homeSandbox) toRows(isLast bool, cont string, uhome string) []homeRow {
	prefix := pterm.Gray("├─ ")
	if isLast {
		prefix = pterm.Gray("└─ ")
	}

	return []homeRow{
		{name: cont + prefix + v.id, status: v.statusDisplay(), kind: mountTypeDisplay(v.mountType), extra: v.extraDisplay(uhome), typ: "sandbox", data: v},
	}
}

type homeMount struct {
	wsPath       string
	specName     string
	harness      string
	profile      string
	mcps         []string
	maxInstances int
	sandboxes    []homeSandbox
}

func (v *homeMount) key() string {
	return v.wsPath + "\x00" + v.specName
}

func (v *homeMount) extraDisplay() string {
	profile := v.profile
	if profile == "" {
		profile = "-"
	}

	facts := []string{"profile: " + profile}
	if len(v.mcps) > 0 {
		facts = append(facts, "mcp: "+strings.Join(v.mcps, ", "))
	}
	return strings.Join(facts, extraSep)
}

func (v *homeMount) toRows(isLast bool, collapsed bool, uhome string) []homeRow {
	prefix, cont := pterm.Gray("├─ "), pterm.Gray("│  ")
	if isLast {
		prefix, cont = pterm.Gray("└─ "), "   "
	}

	name := v.specName
	if len(v.sandboxes) > 0 {
		if collapsed {
			name = pterm.Gray("▸ ") + name
		} else {
			name = pterm.Gray("▾ ") + name
		}
	}

	rows := []homeRow{
		{
			name:   prefix + name,
			status: fmt.Sprintf("%d/%d", len(v.sandboxes), v.maxInstances),
			kind:   v.harness,
			extra:  v.extraDisplay(),
			typ:    "mount",
			data:   v,
		},
	}
	if collapsed {
		return rows
	}

	for i, sb := range v.sandboxes {
		rows = append(rows, sb.toRows(i == len(v.sandboxes)-1, cont, uhome)...)
	}
	return rows
}

type homeWorkspace struct {
	workspace Workspace
	mounts    []homeMount
}

func (v *homeWorkspace) toRows(collapsed map[string]bool, uhome string) []homeRow {
	rows := []homeRow{
		{name: v.workspace.Name(), extra: cutHome(v.workspace.Path(), uhome), typ: "workspace", data: v},
	}
	for i, m := range v.mounts {
		rows = append(rows, m.toRows(i == len(v.mounts)-1, collapsed[m.key()], uhome)...)
	}
	return rows
}

type homeRow struct {
	name   string
	status string
	kind   string
	extra  string
	typ    string
	data   any
}

type homePage struct {
	*basePage

	index      int
	provider   *APIProvider
	workspaces []homeWorkspace
	uhome      string
	collapsed  map[string]bool
	rows       []homeRow
}

func (p *homePage) Focus() tea.Cmd {
	p.basePage.Focus()
	if p.collapsed == nil {
		p.collapsed = make(map[string]bool)
	}
	return p.fetch()
}

func (p *homePage) buildRows() []homeRow {
	rows := []homeRow{
		{name: "NAME", status: "STATUS", kind: "TYPE", extra: "EXTRA", typ: "header"},
	}
	for _, v := range p.workspaces {
		rows = append(rows, v.toRows(p.collapsed, p.uhome)...)
	}
	return rows
}

func (p *homePage) setCollapsed(collapsed bool) (page, tea.Cmd) {
	m, ok := p.rowHomeMount()
	if !ok || len(m.sandboxes) == 0 || p.collapsed[m.key()] == collapsed {
		return p, nil
	}
	p.collapsed[m.key()] = collapsed
	p.rows = p.buildRows()
	return p, p.actions()
}

func (p homePage) fetch() tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		api, err := p.provider.Get()
		if err != nil {
			return homeDataMsg{gen: gen, err: err}
		}

		var workspaces []homeWorkspace
		for _, w := range loadWorkspaces(context.Background(), api) {
			hw := homeWorkspace{workspace: w}

			for _, m := range w.Mounts() {
				spec := m.Spec()
				hm := homeMount{
					wsPath:       w.Path(),
					specName:     spec.Name,
					harness:      string(spec.Harness),
					profile:      spec.Profile,
					mcps:         spec.MCPs,
					maxInstances: spec.MaxInstances,
				}

				for _, s := range m.Sandboxes() {
					sb := s.Sandbox()
					hm.sandboxes = append(hm.sandboxes, homeSandbox{
						wsPath:      w.Path(),
						id:          sb.ID(),
						mountType:   m.Type(),
						sandbox:     sb,
						workDir:     s.WorkDir(),
						hostWorkDir: s.HostWorkDir(),
						isFree:      sb.IsFree(w.Path()),
					})
				}

				slices.SortFunc(hm.sandboxes, func(a, b homeSandbox) int {
					return strings.Compare(a.id, b.id)
				})
				hw.mounts = append(hw.mounts, hm)
			}

			slices.SortFunc(hw.mounts, func(a, b homeMount) int {
				return strings.Compare(a.specName, b.specName)
			})
			workspaces = append(workspaces, hw)
		}

		slices.SortFunc(workspaces, func(a, b homeWorkspace) int {
			return strings.Compare(a.workspace.Name(), b.workspace.Name())
		})
		return homeDataMsg{gen: gen, workspaces: workspaces, uhome: api.Runtime().Platform.UserHomeDir()}
	}
}

func (p homePage) tick() tea.Cmd {
	gen := p.gen
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg {
		return homeTickMsg{gen: gen}
	})
}

func (p homePage) fetchSessions(sb *homeSandbox) tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		sessions := sb.sandbox.Harness().ListSessions(ctx, sb.wsPath, sb.sandbox)
		return homeSessionsMsg{gen: gen, sandbox: sb, sessions: sessions}
	}
}

func (p *homePage) actions() tea.Cmd {
	var out []action

	if _, ok := p.rowHomeWorkspace(); ok {
		out = append(out, actionGotoDir)
	}

	if m, ok := p.rowHomeMount(); ok && len(m.sandboxes) > 0 {
		if p.collapsed[m.key()] {
			out = append(out, actionExpand)
		} else {
			out = append(out, actionCollapse)
		}
		out = append(out, actionPrompt, actionLaunch)
	}

	if _, ok := p.rowHomeSandbox(); ok {
		out = append(out, actionGotoDir, actionPrompt, actionLaunch)
	}

	out = append(out, actionMove, actionQuit)
	return useActions(out...)
}

func (p *homePage) Update(msg tea.Msg) (page, tea.Cmd) {
	switch msg := msg.(type) {
	case homeTickMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		return p, p.fetch()

	case homeDataMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		if msg.err != nil {
			return p, p.tick()
		}

		p.workspaces = msg.workspaces
		p.uhome = msg.uhome
		rows := p.buildRows()
		p.rows = rows

		if p.index <= 0 || p.index >= len(rows) {
			p.index = 1
		}
		return p, tea.Batch(p.tick(), p.actions())

	case tea.KeyPressMsg:
		if !p.focused {
			return p, nil
		}

		switch msg.String() {
		case "j", "down":
			if p.index < len(p.rows)-1 {
				p.index = p.index + 1
			}
			return p, p.actions()

		case "k", "up":
			if p.index > 1 {
				p.index = p.index - 1
			}
			return p, p.actions()

		case "left":
			return p.setCollapsed(true)

		case "right":
			return p.setCollapsed(false)

		case "space":
			m, ok := p.rowHomeMount()
			if !ok || len(m.sandboxes) == 0 {
				return p, nil
			}
			return p.setCollapsed(!p.collapsed[m.key()])

		case "g":
			var dir, banner string
			if w, ok := p.rowHomeWorkspace(); ok {
				dir = w.workspace.Path()
				banner = fmt.Sprintf(
					"You are in main directory %s of the project %s.\nExit the shell or press %s to go back to nestor.",
					pterm.Magenta(dir), pterm.Green(w.workspace.Name()), pterm.Blue("ctrl-D"),
				)
			} else if sb, ok := p.rowHomeSandbox(); ok {
				dir = sb.hostWorkDir
				if dir == "" {
					dir = sb.workDir
				}
				project := sb.wsPath
				if w, ok := p.homeWorkspaceByPath(sb.wsPath); ok {
					project = w.workspace.Name()
				}
				banner = fmt.Sprintf(
					"You are in %s directory %s of the sandbox %s in the project %s.\nExit the shell or press %s to go back to nestor.",
					pterm.Yellow(mountTypeDisplay(sb.mountType)), pterm.Magenta(dir), pterm.Cyan(sb.id), pterm.Green(project), pterm.Blue("ctrl-D"),
				)
			} else {
				return p, nil
			}

			api, err := p.provider.Get()
			if err != nil {
				return p, nil
			}
			shell := LoadConfig(api.Runtime().Platform).Shell

			p.busy = true
			return p, gotoDir(tabHome, shell, dir, banner)
		}

	case execDoneMsg:
		if msg.tab != tabHome {
			return p, nil
		}
		p.busy = false
		return p, nil

	case homeSessionsMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		p.busy = false
		return p, openEditorWithInitial(tabHome, "view-sessions", "sessions-*.yml", msg.sandbox.useSessionsContent(msg.sessions))

	case editorDoneMsg:
		if msg.tab != tabHome {
			return p, nil
		}
		p.busy = false
		return p, nil
	}
	return p, nil
}

func (p *homePage) homeWorkspaceByPath(path string) (*homeWorkspace, bool) {
	for i := range p.workspaces {
		if p.workspaces[i].workspace.Path() == path {
			return &p.workspaces[i], true
		}
	}
	return nil, false
}

func (p *homePage) rowHomeWorkspace() (*homeWorkspace, bool) {
	if !p.focused || p.index <= 0 || p.index >= len(p.rows) {
		return nil, false
	}
	row := p.rows[p.index]
	if row.typ != "workspace" {
		return nil, false
	}
	v, ok := row.data.(*homeWorkspace)
	return v, ok
}

func (p *homePage) rowHomeMount() (*homeMount, bool) {
	if !p.focused || p.index <= 0 || p.index >= len(p.rows) {
		return nil, false
	}
	row := p.rows[p.index]
	if row.typ != "mount" {
		return nil, false
	}
	v, ok := row.data.(*homeMount)
	return v, ok
}

func (p *homePage) rowHomeSandbox() (*homeSandbox, bool) {
	if !p.focused || p.index <= 0 || p.index >= len(p.rows) {
		return nil, false
	}
	row := p.rows[p.index]
	if row.typ != "sandbox" {
		return nil, false
	}
	v, ok := row.data.(*homeSandbox)
	return v, ok
}

func (p *homePage) View() string {
	if len(p.rows) <= 1 {
		return "  There is no workspace detected."
	}

	var nameW, statusW, kindW, extraW int
	for _, v := range p.rows {
		nameW = max(nameW, widthOf(v.name))
		statusW = max(statusW, widthOf(v.status))
		kindW = max(kindW, widthOf(v.kind))
	}

	nameW += 1                                         // pad 1 on the left
	extraW = p.width - 4 - nameW - statusW - kindW - 6 // pad 1 on the right + 4 padding + 3x2 column separator

	var out strings.Builder
	for i, v := range p.rows {
		cols := []string{
			fit(padLeft(v.name, 1), nameW),
			fit(v.status, statusW),
			fit(v.kind, kindW),
			pad(fit(v.extra, extraW), 1),
		}

		row := strings.Join(cols, "  ")
		if p.index == i {
			row = selectedRow(row)
		}

		out.WriteString(spaces(2))
		out.WriteString(row)
		out.WriteString(spaces(2))
		out.WriteRune('\n')
	}
	return out.String()
}
