package tui

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
)

type mcpTickMsg struct{ gen int }

type mcpDataMsg struct {
	gen  int
	mcps []mcpServer
	err  error
}

type mcpToolsMsg struct {
	gen    int
	server *mcpServer
	tools  map[string]string
	err    error
}

type mcpServer struct {
	name   string
	kind   string
	target string
	vars   map[string]string
	mcp    nestor.MCP
}

func (v *mcpServer) useToolsContent(tools map[string]string, mcpYmlPath string) string {
	policy := v.mcp.ToolPolicy()
	names := slices.Sorted(maps.Keys(tools))

	bullets := make(map[string]string, len(names))
	width := 0
	for _, name := range names {
		bullet := "- " + name
		if !policy.PermitTool(name) {
			bullet = "# - " + name
		}
		bullets[name] = bullet
		width = max(width, len(bullet))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# full tool list for mcp %q - all are used by default\n", v.name)
	sb.WriteString("# to limit it, choose tools from this list and add them to this\n")
	fmt.Fprintf(&sb, "# mcp's \"use_tools\" in %s\n\n", mcpYmlPath)
	sb.WriteString("use_tools:\n")

	for _, name := range names {
		bullet := bullets[name]
		line := "  " + bullet
		if desc := strings.Join(strings.Fields(tools[name]), " "); desc != "" {
			line += strings.Repeat(" ", width-len(bullet)) + " # " + desc
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (v *mcpServer) toRows() []mcpRow {
	rows := []mcpRow{
		{name: v.name, kind: v.kind, extra: v.target, typ: "mcp", data: v},
	}

	keys := slices.Sorted(maps.Keys(v.vars))
	for i, k := range keys {
		mv := &mcpVar{key: k, value: v.vars[k], isSecret: isSecretKey(k)}
		row := mcpRow{extra: mv.display(), typ: "var", data: mv}
		if i == len(keys)-1 {
			row.name = pterm.Gray("└─ ") + k
		} else {
			row.name = pterm.Gray("├─ ") + k
		}
		rows = append(rows, row)
	}

	return rows
}

type mcpVar struct {
	key      string
	value    string
	isSecret bool
	revealed bool
}

func (v *mcpVar) display() string {
	if v.isSecret {
		return pterm.Gray("**********")
	}
	return v.value
}

type mcpRow struct {
	name  string
	kind  string
	extra string
	typ   string
	data  any
}

type mcpPage struct {
	*basePage

	index     int
	provider  *APIProvider
	rows      []mcpRow
	verifyErr error

	pendingEditErr     error
	pendingEditContent string
}

func (p *mcpPage) Focus() tea.Cmd {
	p.basePage.Focus()
	return p.fetch()
}

func (p mcpPage) fetch() tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		api, err := p.provider.Get()
		if err != nil {
			return mcpDataMsg{gen: gen, err: err}
		}

		var servers []mcpServer
		for _, m := range api.Runtime().Registry.MCPs() {
			servers = append(servers, mcpServer{
				name:   m.Name(),
				kind:   m.Kind(),
				target: m.Target(),
				vars:   m.Vars(),
				mcp:    m,
			})
		}

		slices.SortFunc(servers, func(a, b mcpServer) int {
			return strings.Compare(a.name, b.name)
		})
		return mcpDataMsg{gen: gen, mcps: servers}
	}
}

func (p mcpPage) tick() tea.Cmd {
	gen := p.gen
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg {
		return mcpTickMsg{gen: gen}
	})
}

func (p mcpPage) fetchTools(s *mcpServer) tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		tools, err := s.mcp.AvailableTools(ctx)
		return mcpToolsMsg{gen: gen, server: s, tools: tools, err: err}
	}
}

func (p *mcpPage) actions() tea.Cmd {
	if p.pendingEditErr != nil {
		return useActions(actionBackToEditor, actionQuit)
	}

	var out []action

	if _, ok := p.rowMCPServer(); ok {
		out = append(out, actionTool)
	}

	if v, ok := p.rowMCPVar(); ok && v.isSecret {
		if v.revealed {
			out = append(out, actionHideSecret)
		} else {
			out = append(out, actionViewSecret)
		}
	}

	out = append(out, actionEditConfiguration, actionMove, actionQuit)
	return useActions(out...)
}

func (p *mcpPage) Update(msg tea.Msg) (page, tea.Cmd) {
	switch msg := msg.(type) {
	case mcpTickMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		return p, p.fetch()

	case mcpDataMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		if msg.err != nil {
			return p, p.tick()
		}

		rows := []mcpRow{
			{name: "NAME", kind: "TYPE", extra: "EXTRA", typ: "header"},
		}
		for _, v := range msg.mcps {
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
			return p, openEditorWithInitial(tabMCP, "edit-mcp-config", "mcp-*.yml", content)
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
			content, err := fs.AtomicReadFile(api.Runtime().Platform.MCPYmlFile())
			if err != nil {
				return p, nil
			}
			p.busy = true
			return p, openEditorWithInitial(tabMCP, "edit-mcp-config", "mcp-*.yml", string(content))

		case "enter":
			mv, ok := p.rowMCPVar()
			if !ok || !mv.isSecret {
				return p, nil
			}
			mv.revealed = !mv.revealed
			return p, p.actions()

		case "t":
			s, ok := p.rowMCPServer()
			if !ok {
				return p, nil
			}
			p.busy = true
			return p, p.fetchTools(s)
		}

	case mcpToolsMsg:
		if p.ignore(msg.gen) {
			return p, nil
		}
		if msg.err != nil {
			p.busy = false
			p.verifyErr = msg.err
			return p, nil
		}
		p.verifyErr = nil
		p.busy = true

		path := ""
		if api, err := p.provider.Get(); err == nil {
			path = api.Runtime().Platform.MCPYmlFile()
		}
		return p, openEditorWithInitial(tabMCP, "view-mcp-tools", "mcp-tools-*.yml", msg.server.useToolsContent(msg.tools, path))

	case editorDoneMsg:
		if msg.tab != tabMCP {
			return p, nil
		}
		p.busy = false
		if msg.tag != "edit-mcp-config" {
			return p, nil
		}
		if msg.err != nil {
			return p, nil
		}

		if _, perr := nestor.ParseMCPs(strings.NewReader(msg.content)); perr != nil {
			p.pendingEditErr = perr
			p.pendingEditContent = msg.content
			return p, p.actions()
		}

		api, err := p.provider.Get()
		if err == nil {
			err = fs.AtomicWriteFile(api.Runtime().Platform.MCPYmlFile(), []byte(msg.content), 0o644)
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

func (p *mcpPage) rowMCPServer() (*mcpServer, bool) {
	if !p.focused || p.index <= 0 || p.index >= len(p.rows) {
		return nil, false
	}
	row := p.rows[p.index]
	if row.typ != "mcp" {
		return nil, false
	}
	s, ok := row.data.(*mcpServer)
	return s, ok
}

func (p *mcpPage) rowMCPVar() (*mcpVar, bool) {
	if !p.focused || p.index <= 0 || p.index >= len(p.rows) {
		return nil, false
	}
	row := p.rows[p.index]
	if row.typ != "var" {
		return nil, false
	}
	v, ok := row.data.(*mcpVar)
	return v, ok
}

func (p *mcpPage) View() string {
	if p.pendingEditErr != nil {
		var out strings.Builder
		out.WriteString("\n  ")
		out.WriteString(pterm.Red("Failed to parse mcp configuration:"))
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

	if len(p.rows) == 1 {
		out.WriteString("  There is no MCP registered. Press ")
		out.WriteString(actionEditConfiguration.Keys())
		out.WriteString(" to register MCP")
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
		if mv, ok := v.data.(*mcpVar); ok && mv.revealed {
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
