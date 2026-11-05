package tui

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
)

type restoredMsg struct {
	n   int
	err error
}

func restore(entries []snapshot.Entry) tea.Cmd {
	return func() tea.Msg {
		session, err := control.Session()
		if err != nil {
			return restoredMsg{err: err}
		}
		live, err := agents.List()
		if err != nil {
			return restoredMsg{err: err}
		}
		n := 0
		for _, e := range entries {
			if _, err := os.Stat(e.Dir); err != nil {
				continue
			}
			if _, err := control.Resume(session, e, "", live); err != nil {
				return restoredMsg{n, err}
			}
			n++
		}
		return restoredMsg{n: n}
	}
}

func (m model) restoreAll() (model, tea.Cmd) {
	if len(m.restorable) == 0 {
		m.actionErr = errors.New("nothing to restore")
		return m, nil
	}
	if m.view == nil {
		m.actionErr = errors.New("run cockpit inside tmux to restore sessions")
		return m, nil
	}
	entries := m.restorable
	m.restorable = nil
	return m, restore(entries)
}

func (m model) checkRestore(live []agents.Live) model {
	if !m.restoreChecked {
		m.restoreChecked = true
		m.restorable = snapshot.Missing(m.saved, live)
	}
	return m
}

func saveSnapshot(m model) error {
	live, err := agents.List()
	if err != nil {
		return err
	}
	return snapshot.Save(append(snapshot.FromLive(live), m.restorable...))
}

func restoredNotice(n int) string {
	if n == 1 {
		return "✓ restored 1 session"
	}
	return fmt.Sprintf("✓ restored %d sessions", n)
}

func cmdRestore(m model, _ string) (model, tea.Cmd) {
	return m.restoreAll()
}
