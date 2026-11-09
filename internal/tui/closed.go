package tui

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

const closedRowHeight = 2

func cmdClosed(m model, _ string) (model, tea.Cmd) {
	m.overlay, m.closed, m.closedCursor = "closed", state.LoadClosed(), 0
	return m, nil
}

func (m model) updateClosed(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	n := len(m.closed)
	switch msg.String() {
	case "j", "down":
		m.closedCursor = min(m.closedCursor+1, max(0, n-1))
	case "k", "up":
		m.closedCursor = max(m.closedCursor-1, 0)
	case "enter", "right", "l":
		if n > 0 {
			return m.resumeClosed(m.closed[m.closedCursor])
		}
	case "d", "x":
		if n > 0 {
			m.actionErr = state.RemoveClosed(m.closed[m.closedCursor])
			m.closed = slices.Delete(slices.Clone(m.closed), m.closedCursor, m.closedCursor+1)
			m.closedCursor = min(m.closedCursor, max(0, len(m.closed)-1))
		}
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.overlay = ""
	}
	return m, nil
}

func (m model) resumeClosed(c state.Closed) (tea.Model, tea.Cmd) {
	m.overlay = ""
	_, missing := os.Stat(c.Dir)
	switch {
	case c.SessionID == "":
		m.actionErr = errors.New(closedName(c) + " has no session to resume")
	case missing != nil:
		m.actionErr = errors.New(paths.Contract(c.Dir) + " no longer exists")
	case m.view == nil:
		m.actionErr = errors.New("run cockpit inside tmux to resume sessions")
	default:
		m.actionErr = state.RemoveClosed(c)
		return m, restore([]snapshot.Entry{{Kind: c.Kind, Dir: c.Dir, Name: c.Name, Label: c.Label, SessionID: c.SessionID, Mode: c.Mode}})
	}
	return m, nil
}

func closedName(c state.Closed) string {
	return cmp.Or(c.Label, c.Name, filepath.Base(c.Dir))
}

func (m model) closedView(height int) string {
	rows := []string{" " + titleStyle.Render(withIcon(icons.restore, "recently closed")) + mutedStyle.Render("  ⏎ resume · d remove · esc close")}
	if len(m.closed) == 0 {
		rows = append(rows, "", mutedStyle.Render("   Nothing closed yet."))
	}
	fit := max(1, (height-1)/closedRowHeight)
	start := max(0, m.closedCursor-fit+1)
	for i, c := range m.closed[start:min(len(m.closed), start+fit)] {
		rows = append(rows, m.closedRows(start+i, c)...)
	}
	for i, r := range rows {
		rows[i] = ansi.Truncate(r, m.width, "…")
	}
	return strings.Join(rows, "\n")
}

func (m model) closedRows(i int, c state.Closed) []string {
	meta := []string{paths.Contract(c.Dir)}
	if c.Parent != "" {
		meta = append(meta, "child of "+c.Parent)
	}
	if c.CostUSD > 0 {
		meta = append(meta, withIcon(icons.cost, fmt.Sprintf("%.2f", c.CostUSD)))
	}
	meta = append(meta, "closed "+age(time.Since(c.ClosedAt))+" ago")
	cursor, style := "   ", mutedStyle
	if i == m.closedCursor {
		cursor, style = " "+barStyle.Render("┃")+" ", nameStyle
	}
	detail := orPlaceholder(tldr(c.Message), "no reply")
	if c.Diff != "" {
		detail += subtleStyle.Render("  · " + c.Diff)
	}
	return []string{
		cursor + style.Render(withIcon(icons.kinds[c.Kind], closedName(c))) + "  " + subtleStyle.Render(strings.Join(meta, " · ")),
		cursor + "  " + detail,
	}
}
