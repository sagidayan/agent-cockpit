package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
	"github.com/sagidayan/agent-cockpit/internal/transcript"
)

const (
	recentMessages = 40
	paneLines      = 200
	detailHead     = 3
	wheelStep      = 3
)

type detail struct {
	open     bool
	top      int
	id       string
	conv     []transcript.Message
	screen   string
	fromPane bool
}

type detailMsg struct {
	id       string
	conv     []transcript.Message
	screen   string
	fromPane bool
	err      error
}

var roleMarkers = map[string]string{
	transcript.User:      keyStyle.Render("❯"),
	transcript.Assistant: mutedStyle.Render("⏺"),
	transcript.Tool:      subtleStyle.Render("⎿"),
}

func loadDetail(a agents.Live) tea.Cmd {
	return func() tea.Msg {
		if a.Kind == "claude" {
			if conv, ok := transcript.Conversation(transcript.File(a.Agent), recentMessages); ok {
				return detailMsg{id: a.ID, conv: conv}
			}
		}
		screen, err := tmux.Capture(a.ID, paneLines)
		return detailMsg{id: a.ID, screen: screen, fromPane: true, err: err}
	}
}

func (m model) reloadDetail() tea.Cmd {
	a, ok := m.selected()
	if !m.detail.open || !ok {
		return nil
	}
	return loadDetail(a)
}

func (m model) openDetail() (model, tea.Cmd) {
	m.detail.open, m.detail.top = true, 0
	return m, m.reloadDetail()
}

func (m model) setDetail(msg detailMsg) model {
	if a, ok := m.selected(); !ok || a.ID != msg.id {
		return m
	}
	if msg.err != nil {
		m.actionErr = msg.err
	}
	if msg.id != m.detail.id {
		m.detail.top = 0
	}
	m.detail.id, m.detail.conv, m.detail.screen, m.detail.fromPane = msg.id, msg.conv, msg.screen, msg.fromPane
	return m
}

func (m model) updateDetail(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "i":
		m.detail.open = false
		return m, nil
	case "j", "down":
		return m.scrollDetail(1), nil
	case "k", "up":
		return m.scrollDetail(-1), nil
	case "pgdown":
		return m.scrollDetail(m.detailHeight()), nil
	case "pgup":
		return m.scrollDetail(-m.detailHeight()), nil
	case "home":
		m.detail.top = 0
		return m, nil
	case "end":
		m.detail.top = m.detailMaxTop()
		return m, nil
	}
	next, cmd := m.updateList(msg)
	return next, tea.Batch(cmd, next.(model).reloadDetail())
}

func (m model) wheelDetail(msg tea.MouseMsg) model {
	wheel, ok := msg.(tea.MouseWheelMsg)
	switch {
	case !ok:
	case wheel.Button == tea.MouseWheelUp:
		return m.scrollDetail(-wheelStep)
	case wheel.Button == tea.MouseWheelDown:
		return m.scrollDetail(wheelStep)
	}
	return m
}

func (m model) scrollDetail(n int) model {
	m.detail.top = min(max(0, m.detail.top+n), m.detailMaxTop())
	return m
}

func (m model) detailMaxTop() int {
	a, ok := m.selected()
	if !ok {
		return 0
	}
	return max(0, len(m.detailLines(a, m.detailWidth()))-m.detailHeight())
}

func (m model) detailWidth() int {
	return max(m.width-4, 10)
}

func (m model) detailHeight() int {
	return max(m.listHeight()-detailHead, 1)
}

func (m model) detailView() string {
	a, ok := m.selected()
	if !ok {
		return mutedStyle.Render(" No agent selected.")
	}
	width, height := m.detailWidth(), m.detailHeight()
	body := m.detailLines(a, width)
	top := min(m.detail.top, max(0, len(body)-height))
	shown := body[top:min(len(body), top+height)]
	var rows []string
	for _, l := range m.cardHead(a, width) {
		rows = append(rows, m.highlight(" "+barStyle.Render("┃")+" "+l))
	}
	rows = append(rows, m.detailRule(fmt.Sprintf(" %d-%d of %d", top+1, top+len(shown), len(body))))
	for _, l := range shown {
		rows = append(rows, "   "+l)
	}
	return strings.Join(rows, "\n")
}

func (m model) detailRule(position string) string {
	return " " + subtleStyle.Render(strings.Repeat("─", max(0, m.width-2-lipgloss.Width(position)))) + mutedStyle.Render(position)
}

func (m model) detailLines(a agents.Live, width int) []string {
	rows := []string{titleStyle.Render("last prompt")}
	rows = append(rows, wrapped(orPlaceholder(a.Prompt, "no prompt yet"), width)...)
	rows = append(rows, "", titleStyle.Render("last reply"))
	rows = append(rows, wrapped(orPlaceholder(a.Message, "no reply yet"), width)...)
	return append(append(rows, ""), m.history(a, width)...)
}

func (m model) history(a agents.Live, width int) []string {
	switch {
	case m.detail.id != a.ID:
		return []string{titleStyle.Render("recent conversation"), subtleStyle.Render("loading…")}
	case m.detail.fromPane:
		return append([]string{titleStyle.Render("pane")}, wrapped(strings.TrimRight(m.detail.screen, " \n"), width)...)
	}
	rows := []string{titleStyle.Render("recent conversation")}
	for _, msg := range m.detail.conv {
		rows = append(rows, message(msg, width)...)
	}
	return rows
}

func message(msg transcript.Message, width int) []string {
	text := msg.Text
	if msg.Role == transcript.Tool {
		text = subtleStyle.Render(text)
	}
	lines := wrapped(text, width-2)
	for i := range lines {
		indent := "  "
		if i == 0 {
			indent = roleMarkers[msg.Role] + " "
		}
		lines[i] = indent + lines[i]
	}
	if msg.Role == transcript.User {
		return append([]string{""}, lines...)
	}
	return lines
}

func wrapped(s string, width int) []string {
	return strings.Split(ansi.Wrap(s, width, ""), "\n")
}
