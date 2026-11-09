package tui

import (
	"cmp"
	"errors"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

func arrange(order []string, live []agents.Live) []string {
	ids := make([]string, 0, len(live))
	for _, a := range live {
		ids = append(ids, a.ID)
	}
	kept := slices.DeleteFunc(slices.Clone(order), func(id string) bool { return !slices.Contains(ids, id) })
	for _, id := range ids {
		if !slices.Contains(kept, id) {
			kept = append(kept, id)
		}
	}
	return kept
}

func byOrder(live []agents.Live, order []string) {
	slices.SortStableFunc(live, func(a, b agents.Live) int {
		return cmp.Compare(slices.Index(order, a.ID), slices.Index(order, b.ID))
	})
}

func (m model) shift(step int) (model, tea.Cmd) {
	a, ok := m.selected()
	if !ok {
		return m, nil
	}
	if sorters[m.sortBy] != nil {
		m.actionErr = errors.New("run :sort location to reorder by hand")
		return m, nil
	}
	b, ok := m.sibling(a, step)
	if !ok {
		return m, nil
	}
	m.order = slices.Clone(m.order)
	i, j := slices.Index(m.order, a.ID), slices.Index(m.order, b.ID)
	m.order[i], m.order[j] = m.order[j], m.order[i]
	m.all = slices.Clone(m.all)
	byOrder(m.all, m.order)
	order := m.order
	return m.refreshView(), func() tea.Msg { return actionMsg{err: state.SaveOrder(order)} }
}

func (m model) sibling(a agents.Live, step int) (agents.Live, bool) {
	d := m.depth[a.ID]
	for j := m.cursor + step; j >= 0 && j < len(m.agents); j += step {
		b := m.agents[j]
		switch {
		case m.depth[b.ID] < d:
			return agents.Live{}, false
		case m.depth[b.ID] == d && m.favoriteRank(b) == m.favoriteRank(a):
			return b, true
		}
	}
	return agents.Live{}, false
}
