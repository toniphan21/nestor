package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type homePage struct{}

func (t homePage) Init() tea.Cmd {
	return nil
}

func (t homePage) Update(tea.Msg) (homePage, tea.Cmd) {
	return t, nil
}

func (t homePage) View() string {
	sb := strings.Builder{}
	for i := range 200 {
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (t homePage) busy() bool {
	return false
}
