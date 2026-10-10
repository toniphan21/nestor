package tui

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
)

type specTickMsg struct{ gen int }

type specDataMsg struct {
	gen   int
	specs []specEntry
	uhome string
	err   error
}

func cutHome(path, uhome string) string {
	if uhome == "" {
		return path
	}
	if rel, ok := strings.CutPrefix(path, uhome); ok {
		return "~" + rel
	}
	return path
}

type specVar struct {
	key      string
	value    string
	isSecret bool
	revealed bool
}

func (v *specVar) display() string {
	if v.isSecret {
		return pterm.Gray("**********")
	}
	return v.value
}

type specMount struct {
	mtype    string
	path     string
	at       string
	readOnly bool
}

type specEntry struct {
	name         string
	harness      string
	profile      string
	target       string
	maxInstances int
	stateScope   string
	mcps         []string
	mounts       []specMount
	env          map[string]string
}

func (v *specEntry) stateScopeDisplay() string {
	if v.stateScope == "instance" {
		return "per-instance"
	}
	return v.stateScope
}

func (v *specEntry) extraDisplay() string {
	extra := "target: " + v.target
	if len(v.mcps) > 0 {
		extra += extraSep + "mcps: " + strings.Join(v.mcps, ", ")
	}
	extra += extraSep + "max-instances: " + strconv.Itoa(v.maxInstances) +
		extraSep + "state-scope: " + v.stateScopeDisplay()
	return extra
}

func (v *specEntry) toRows(uhome string) []specRow {
	rows := []specRow{
		{name: v.name, kind: v.harness, profile: v.profile, extra: v.extraDisplay(), typ: "specEntry", data: v},
	}

	for i, m := range v.mounts {
		readOnly := "read-write"
		if m.readOnly {
			readOnly = "read-only"
		}

		at := m.at
		if at == "" {
			at = m.path
		}
		value := cutHome(m.path, uhome) + " → " + cutHome(at, uhome)

		row := specRow{
			kind:    strings.TrimPrefix(m.mtype, "git_"),
			profile: readOnly,
			extra:   value,
			typ:     "specMount",
			data:    &v.mounts[i],
		}
		name := "mount[" + strconv.Itoa(i) + "]"
		if i == len(v.mounts)-1 {
			row.name = pterm.Gray("└─ ") + name
		} else {
			row.name = pterm.Gray("├─ ") + name
		}
		rows = append(rows, row)
	}

	envKeys := slices.Sorted(maps.Keys(v.env))
	for i, k := range envKeys {
		mv := &specVar{key: k, value: v.env[k], isSecret: isSecretKey(k)}
		row := specRow{kind: pterm.Gray("env"), extra: mv.display(), typ: "specEnv", data: mv}
		if i == len(envKeys)-1 {
			row.name = pterm.Gray("└─ ") + k
		} else {
			row.name = pterm.Gray("├─ ") + k
		}
		rows = append(rows, row)
	}

	return rows
}

type specRow struct {
	name    string
	kind    string
	profile string
	extra   string
	typ     string
	data    any
}

type specPage struct {
	*basePage

	index     int
	provider  *APIProvider
	rows      []specRow
	verifyErr error

	pendingEditErr     error
	pendingEditContent string
}

func (p *specPage) Focus() tea.Cmd {
	p.basePage.Focus()
	return p.fetch()
}

func (p specPage) fetch() tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		api, err := p.provider.Get()
		if err != nil {
			return specDataMsg{gen: gen, err: err}
		}

		var specs []specEntry
		for _, sp := range api.Runtime().Registry.SandboxSpecs() {
			var mounts []specMount
			for _, m := range sp.Mounts {
				mounts = append(mounts, specMount{mtype: string(m.Type), path: m.Path, at: m.At, readOnly: m.ReadOnly})
			}

			specs = append(specs, specEntry{
				name:         sp.Name,
				harness:      string(sp.Harness),
				profile:      sp.Profile,
				target:       sp.Target,
				maxInstances: sp.MaxInstances,
				stateScope:   string(sp.StateScope),
				mcps:         sp.MCPs,
				mounts:       mounts,
				env:          sp.Env,
			})
		}

		slices.SortFunc(specs, func(a, b specEntry) int {
			return strings.Compare(a.name, b.name)
		})
		return specDataMsg{gen: gen, specs: specs, uhome: api.Runtime().Platform.UserHomeDir()}
	}
}

func (p specPage) tick() tea.Cmd {
	gen := p.gen
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg {
		return specTickMsg{gen: gen}
	})
}

func (p *specPage) actions() tea.Cmd {
	if p.pendingEditErr != nil {
		return useActions(actionBackToEditor, actionQuit)
	}

	var out []action

	if v, ok := p.rowSpecVar(); ok && v.isSecret {
		if v.revealed {
			out = append(out, actionHideSecret)
		} else {
			out = append(out, actionViewSecret)
		}
	}

	out = append(out, actionEditConfiguration, actionMove, actionQuit)
	return useActions(out...)
}

func (p *specPage) Update(msg tea.Msg) (page, tea.Cmd) {
	switch msg := msg.(type) {
	case specTickMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		return p, p.fetch()

	case specDataMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		if msg.err != nil {
			return p, p.tick()
		}

		rows := []specRow{
			{name: "NAME", kind: "TYPE", profile: "PROFILE", extra: "EXTRA", typ: "header"},
		}
		for _, v := range msg.specs {
			rows = append(rows, v.toRows(msg.uhome)...)
		}
		p.rows = rows

		if p.index <= 0 || p.index >= len(rows) {
			p.index = 1
		}
		return p, tea.Batch(p.tick(), p.actions())

	case tea.KeyPressMsg:
		if !p.focused {
			return p, nil
		}

		if p.pendingEditErr != nil {
			if msg.String() != "enter" {
				return p, nil
			}
			content := p.pendingEditContent
			p.pendingEditErr = nil
			p.pendingEditContent = ""
			p.busy = true
			return p, openEditorWithInitial(tabSpec, "edit-spec-config", "sandbox-*.yml", content)
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

		case "e":
			api, err := p.provider.Get()
			if err != nil {
				return p, nil
			}
			content, err := fs.AtomicReadFile(api.Runtime().Platform.SandboxYmlFile())
			if err != nil {
				return p, nil
			}
			p.busy = true
			return p, openEditorWithInitial(tabSpec, "edit-spec-config", "sandbox-*.yml", string(content))

		case "enter":
			mv, ok := p.rowSpecVar()
			if !ok || !mv.isSecret {
				return p, nil
			}
			mv.revealed = !mv.revealed
			return p, p.actions()
		}

	case editorDoneMsg:
		if msg.tab != tabSpec {
			return p, nil
		}
		p.busy = false
		if msg.tag != "edit-spec-config" {
			return p, nil
		}
		if msg.err != nil {
			return p, nil
		}

		api, err := p.provider.Get()
		if err != nil {
			p.verifyErr = err
			return p, nil
		}

		if _, perr := nestor.ParseSandboxSpecs(api.Runtime(), strings.NewReader(msg.content)); perr != nil {
			p.pendingEditErr = perr
			p.pendingEditContent = msg.content
			return p, p.actions()
		}

		if err := fs.AtomicWriteFile(api.Runtime().Platform.SandboxYmlFile(), []byte(msg.content), 0o644); err != nil {
			p.verifyErr = err
			return p, p.fetch()
		}

		if err := p.provider.Verify(); err == nil {
			p.provider.Reset()
			p.verifyErr = nil
		} else {
			p.verifyErr = err
		}
		return p, p.fetch()
	}
	return p, nil
}

func (p *specPage) rowSpecVar() (*specVar, bool) {
	if !p.focused || p.index <= 0 || p.index >= len(p.rows) {
		return nil, false
	}
	row := p.rows[p.index]
	if row.typ != "specEnv" {
		return nil, false
	}
	v, ok := row.data.(*specVar)
	return v, ok
}

func (p *specPage) View() string {
	if p.pendingEditErr != nil {
		var out strings.Builder
		out.WriteString("\n  ")
		out.WriteString(pterm.Red("Failed to parse sandbox configuration:"))
		out.WriteString("\n\n")
		for _, line := range strings.Split(p.pendingEditErr.Error(), "\n") {
			out.WriteString("  ")
			out.WriteString(pterm.Red(line))
			out.WriteString("\n")
		}
		out.WriteString("\n  ")
		out.WriteString(pterm.Gray("press "))
		out.WriteString(actionBackToEditor.Keys())
		out.WriteString(pterm.Gray(" to go back to the editor"))
		return out.String()
	}

	var out strings.Builder
	if p.verifyErr != nil {
		out.WriteString("  ")
		out.WriteString(pterm.Red(p.verifyErr.Error()))
		out.WriteString("\n\n")
	}

	if len(p.rows) <= 1 {
		out.WriteString("  There is no sandbox spec registered. Press ")
		out.WriteString(actionEditConfiguration.Keys())
		out.WriteString(" to register a sandbox spec")
		return out.String()
	}

	var nameW, kindW, profileW, extraW int
	for _, v := range p.rows {
		nameW = max(nameW, widthOf(v.name))
		kindW = max(kindW, widthOf(v.kind))
		profileW = max(profileW, widthOf(v.profile))
	}

	nameW += 1                                          // pad 1 on the left
	extraW = p.width - 4 - nameW - kindW - profileW - 6 // pad 1 on the right + 4 padding + 3x2 column separator

	for i, v := range p.rows {
		extra := v.extra
		if mv, ok := v.data.(*specVar); ok && mv.revealed {
			extra = mv.value
		}

		row := strings.Join([]string{
			fit(padLeft(v.name, 1), nameW),
			fit(v.kind, kindW),
			fit(v.profile, profileW),
			pad(fit(extra, extraW), 1),
		}, "  ")

		if p.index == i {
			row = pterm.NewStyle(pterm.BgGray).Sprint(row)
		}

		out.WriteString(spaces(2))
		out.WriteString(row)
		out.WriteString(spaces(2))
		out.WriteRune('\n')
	}
	return out.String()
}
