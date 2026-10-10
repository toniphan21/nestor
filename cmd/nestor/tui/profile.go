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

type profileTickMsg struct{ gen int }

type profileDataMsg struct {
	gen      int
	profiles []profileEntry
	err      error
}

type profileEntry struct {
	name          string
	proxy         bool
	auth          string
	models        int
	targets       []string
	defaultTarget string
	defaultModel  string
	settings      map[string]string
	options       map[string]string
}

// isUnfilledPlaceholder reports whether value still looks like the "your ..."
// placeholder text assets/profile.yml ships for secret settings/options
// (e.g. "your oauth token", "your api key") instead of a real value.
func isUnfilledPlaceholder(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "your ")
}

type profileVar struct {
	key      string
	value    string
	isSecret bool
	revealed bool
}

func (v *profileVar) display() string {
	if v.isSecret {
		return pterm.Gray("**********")
	}
	return v.value
}

func (v *profileEntry) targetsDisplay() string {
	out := make([]string, len(v.targets))
	for i, t := range v.targets {
		if t == v.defaultTarget {
			out[i] = pterm.Blue(t)
		} else {
			out[i] = pterm.White(t)
		}
	}
	return strings.Join(out, ", ")
}

func (v *profileEntry) proxyDisplay() string {
	if v.proxy {
		return pterm.Green("yes")
	}
	return pterm.Yellow("no ")
}

func (v *profileEntry) extraDisplay() string {
	return "proxy: " + v.proxyDisplay() +
		extraSep + "targets: " + v.targetsDisplay() +
		extraSep + "models: " + strconv.Itoa(v.models) + ", default: " + v.defaultModel
}

func (v *profileEntry) toRows() []profileRow {
	rows := []profileRow{
		{name: v.name, kind: v.auth, extra: v.extraDisplay(), typ: "profile", data: v},
	}

	settingKeys := slices.Sorted(maps.Keys(v.settings))
	for i, k := range settingKeys {
		mv := &profileVar{key: k, value: v.settings[k], isSecret: isSecretKey(k)}
		extra := mv.display()
		if mv.isSecret && isUnfilledPlaceholder(mv.value) {
			mv.isSecret = false // nothing to hide, it's just unfilled template text
			extra = pterm.Red("⚠ " + mv.value + " - fill this in")
		}
		row := profileRow{kind: pterm.Gray("setting"), extra: extra, typ: "profileSetting", data: mv}
		if i == len(settingKeys)-1 {
			row.name = pterm.Gray("└─ ") + k
		} else {
			row.name = pterm.Gray("├─ ") + k
		}
		rows = append(rows, row)
	}

	optionKeys := slices.Sorted(maps.Keys(v.options))
	for i, k := range optionKeys {
		mv := &profileVar{key: k, value: v.options[k], isSecret: isSecretKey(k)}
		extra := mv.display()
		if mv.isSecret && isUnfilledPlaceholder(mv.value) {
			mv.isSecret = false // nothing to hide, it's just unfilled template text
			extra = pterm.Red("⚠ " + mv.value + " - fill this in")
		}
		row := profileRow{kind: pterm.Gray("option"), extra: extra, typ: "profileOptions", data: mv}
		if i == len(optionKeys)-1 {
			row.name = pterm.Gray("└─ ") + k
		} else {
			row.name = pterm.Gray("├─ ") + k
		}
		rows = append(rows, row)
	}

	return rows
}

type profileRow struct {
	name  string
	kind  string
	extra string
	typ   string
	data  any
}

type profilePage struct {
	*basePage

	index     int
	provider  *APIProvider
	rows      []profileRow
	verifyErr error

	pendingEditErr     error
	pendingEditContent string
}

func (p *profilePage) Focus() tea.Cmd {
	p.basePage.Focus()
	return p.fetch()
}

func (p profilePage) fetch() tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		api, err := p.provider.Get()
		if err != nil {
			return profileDataMsg{gen: gen, err: err}
		}

		var profiles []profileEntry
		for _, pr := range api.Runtime().Registry.Profiles() {
			profiles = append(profiles, profileEntry{
				name:          pr.Name,
				proxy:         pr.Proxy,
				auth:          string(pr.Auth),
				models:        len(pr.Models),
				targets:       pr.Targets,
				defaultTarget: pr.DefaultTarget,
				defaultModel:  pr.DefaultModel,
				settings:      pr.Settings,
				options:       pr.Options,
			})
		}

		slices.SortFunc(profiles, func(a, b profileEntry) int {
			return strings.Compare(a.name, b.name)
		})
		return profileDataMsg{gen: gen, profiles: profiles}
	}
}

func (p profilePage) tick() tea.Cmd {
	gen := p.gen
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg {
		return profileTickMsg{gen: gen}
	})
}

func (p *profilePage) actions() tea.Cmd {
	if p.pendingEditErr != nil {
		return useActions(actionBackToEditor, actionQuit)
	}

	var out []action

	if v, ok := p.rowProfileVar(); ok && v.isSecret {
		if v.revealed {
			out = append(out, actionHideSecret)
		} else {
			out = append(out, actionViewSecret)
		}
	}

	out = append(out, actionEditConfiguration, actionMove, actionQuit)
	return useActions(out...)
}

func (p *profilePage) Update(msg tea.Msg) (page, tea.Cmd) {
	switch msg := msg.(type) {
	case profileTickMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		return p, p.fetch()

	case profileDataMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		if msg.err != nil {
			return p, p.tick()
		}

		rows := []profileRow{
			{name: "NAME", kind: "TYPE", extra: "EXTRA", typ: "header"},
		}
		for _, v := range msg.profiles {
			rows = append(rows, v.toRows()...)
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
			return p, openEditorWithInitial(tabProfile, "edit-profile-config", "profile-*.yml", content)
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
			content, err := fs.AtomicReadFile(api.Runtime().Platform.ProfileYmlFile())
			if err != nil {
				return p, nil
			}
			p.busy = true
			return p, openEditorWithInitial(tabProfile, "edit-profile-config", "profile-*.yml", string(content))

		case "enter":
			mv, ok := p.rowProfileVar()
			if !ok || !mv.isSecret {
				return p, nil
			}
			mv.revealed = !mv.revealed
			return p, p.actions()
		}

	case editorDoneMsg:
		if msg.tab != tabProfile {
			return p, nil
		}
		p.busy = false
		if msg.tag != "edit-profile-config" {
			return p, nil
		}
		if msg.err != nil {
			return p, nil
		}

		if _, perr := nestor.ParseProfiles(strings.NewReader(msg.content)); perr != nil {
			p.pendingEditErr = perr
			p.pendingEditContent = msg.content
			return p, p.actions()
		}

		api, err := p.provider.Get()
		if err == nil {
			err = fs.AtomicWriteFile(api.Runtime().Platform.ProfileYmlFile(), []byte(msg.content), 0o644)
		}
		if err != nil {
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

func (p *profilePage) rowProfileVar() (*profileVar, bool) {
	if !p.focused || p.index <= 0 || p.index >= len(p.rows) {
		return nil, false
	}
	row := p.rows[p.index]
	if row.typ != "profileSetting" && row.typ != "profileOptions" {
		return nil, false
	}
	v, ok := row.data.(*profileVar)
	return v, ok
}

func (p *profilePage) View() string {
	if p.pendingEditErr != nil {
		var out strings.Builder
		out.WriteString("\n  ")
		out.WriteString(pterm.Red("Failed to parse profile configuration:"))
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
		out.WriteString("  There is no profile registered. Press ")
		out.WriteString(actionEditConfiguration.Keys())
		out.WriteString(" to register profile")
		return out.String()
	}

	var nameW, kindW, extraW int
	for _, v := range p.rows {
		nameW = max(nameW, widthOf(v.name))
		kindW = max(kindW, widthOf(v.kind))
	}

	nameW += 1                               // pad 1 on the left
	extraW = p.width - 4 - nameW - kindW - 4 // pad 1 on the right + 4 padding + 2x2 column separator

	for i, v := range p.rows {
		extra := v.extra
		if mv, ok := v.data.(*profileVar); ok && mv.revealed {
			extra = mv.value
		}

		cols := []string{
			fit(padLeft(v.name, 1), nameW),
			fit(v.kind, kindW),
			pad(fit(extra, extraW), 1),
		}

		row := strings.Join(cols, "  ")
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
