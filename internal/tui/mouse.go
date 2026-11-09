package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const doubleClick = 400 * time.Millisecond

func (m model) cardAt(y int) (int, bool) {
	line := y - lipgloss.Height(m.header())
	height := m.listHeight()
	if m.overlay != "" || line < 0 || line >= height {
		return 0, false
	}
	cards := m.cards()
	line += m.scrollStart(cards, height)
	for i, c := range cards {
		h := lipgloss.Height(c)
		if line == 0 && m.startsGroup(i) {
			return 0, false
		}
		if line < h {
			return i, true
		}
		line -= h + cardGap
		if line < 0 {
			return 0, false
		}
	}
	return 0, false
}

func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.spawning || m.sending || m.prompt != 0 || m.confirm != nil {
		return m, nil
	}
	if m.detail.open {
		return m.wheelDetail(msg), nil
	}
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		return m.wheel(msg.Button), nil
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m.click(msg.Y, time.Now()), nil
		}
	}
	return m, nil
}

func (m model) wheel(b tea.MouseButton) model {
	switch b {
	case tea.MouseWheelUp:
		m.cursor = max(m.cursor-1, 0)
	case tea.MouseWheelDown:
		m.cursor = min(m.cursor+1, max(0, len(m.agents)-1))
	default:
		return m
	}
	return m.showSelected()
}

func (m model) click(y int, now time.Time) model {
	if m.overlay != "" {
		m.overlay = ""
		return m
	}
	i, ok := m.cardAt(y)
	if !ok {
		return m
	}
	double := i == m.cursor && now.Sub(m.clickedAt) < doubleClick
	m.cursor, m.clickedAt = i, now
	if double {
		return m.interact()
	}
	return m.showSelected()
}
