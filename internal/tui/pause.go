package tui

import (
	"errors"
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func pausePlan(scope []agents.Live, pauseFirst bool) (resume bool, targets []agents.Live) {
	working := withStatus(scope, state.Working)
	paused := withStatus(scope, state.Paused)
	if len(paused) > 0 && (!pauseFirst || len(working) == 0) {
		return true, paused
	}
	return false, working
}

func withStatus(list []agents.Live, s state.Status) []agents.Live {
	return slices.DeleteFunc(slices.Clone(list), func(a agents.Live) bool { return a.Status != s })
}

func (m model) pauseSelected() (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		return m.togglePause(agents.Subtree(m.all, a), false)
	})
}

func (m model) pauseAll() (model, tea.Cmd) {
	return m.togglePause(m.all, true)
}

func (m model) togglePause(scope []agents.Live, pauseFirst bool) (model, tea.Cmd) {
	resume, targets := pausePlan(scope, pauseFirst)
	switch {
	case len(targets) == 0:
		m.notice = "nothing working to pause"
		return m, nil
	case resume:
		return m, apply("resumed", targets, control.Continue)
	}
	return m, apply("paused", targets, control.Pause)
}

func apply(verb string, targets []agents.Live, act func(agents.Live) error) tea.Cmd {
	return tea.Sequence(func() tea.Msg {
		var errs []error
		n := 0
		for _, a := range targets {
			if !tmux.Owned(a.ID) {
				continue
			}
			if err := act(a); err != nil {
				errs = append(errs, err)
				continue
			}
			n++
		}
		return actionMsg{err: errors.Join(errs...), notice: fmt.Sprintf("✓ %s %d agent(s)", verb, n)}
	}, load)
}
