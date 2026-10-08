package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type dockerTickMsg struct{ gen int }

type dockerDataMsg struct {
	gen int
}

type dockerPage struct {
	busy    bool
	focused bool
	gen     int
}

func (p *dockerPage) Focus() tea.Cmd {
	p.focused = true
	p.gen++
	return p.fetch()
}

func (p *dockerPage) Blur() {
	p.focused = false
	p.gen++
}

func (p *dockerPage) Busy() bool {
	return p.busy
}

func (p dockerPage) fetch() tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		return dockerDataMsg{gen: gen}
	}
}

func (p dockerPage) tick() tea.Cmd {
	gen := p.gen
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return dockerTickMsg{gen: gen}
	})
}

func (p *dockerPage) Update(msg tea.Msg) (page, tea.Cmd) {
	switch msg := msg.(type) {
	case dockerTickMsg:
		if msg.gen != p.gen || !p.focused {
			return p, nil
		}
		return p, p.fetch()

	case dockerDataMsg:
		if msg.gen != p.gen {
			return p, nil
		}

		return p, p.tick()
	}
	return p, nil
}

func (p *dockerPage) View() string {
	return "docker"
}
