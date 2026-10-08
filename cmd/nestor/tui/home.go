package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type homeTickMsg struct{ gen int }

type homeDataMsg struct {
	gen   int
	count int
}

type homePage struct {
	*basePage

	count int
}

func (p *homePage) Focus() tea.Cmd {
	p.basePage.Focus()

	return p.fetch()
}

func (p homePage) fetch() tea.Cmd {
	gen := p.gen
	count := p.count
	return func() tea.Msg {
		return homeDataMsg{gen: gen, count: count + 1}
	}
}

func (p homePage) tick() tea.Cmd {
	gen := p.gen
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return homeTickMsg{gen: gen}
	})
}

func (p *homePage) Update(msg tea.Msg) (page, tea.Cmd) {
	switch msg := msg.(type) {
	case homeTickMsg:
		if msg.gen != p.gen || !p.focused {
			return p, nil
		}
		return p, p.fetch()

	case homeDataMsg:
		if msg.gen != p.gen {
			return p, nil
		}
		p.count = msg.count
		return p, p.tick()
	}
	return p, nil
}

func (p *homePage) View() string {
	sb := strings.Builder{}
	for i := range p.count {
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\n")
	}
	return sb.String()
}
