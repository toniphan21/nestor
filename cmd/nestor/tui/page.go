package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/pterm/pterm"
)

type basePage struct {
	busy    bool
	focused bool
	gen     int
	width   int
	height  int
}

func (p *basePage) Focus() {
	p.focused = true
	p.gen++
}

func (p *basePage) Blur() {
	p.focused = false
	p.gen++
}

func (p *basePage) Busy() bool {
	return p.busy
}

func (p *basePage) SetSize(w int, h int) {
	p.width = w
	p.height = h
}

func (p *basePage) ignore(gen int) bool {
	return gen != p.gen || !p.focused
}

func (p *basePage) useActions(actions ...action) tea.Cmd {
	var out []action
	for i, v := range actions {
		v.order = i
		v.keySeparator = pterm.Gray("/")
		out = append(out, v)
	}
	return send(setActionsMsg{actions: out})
}
