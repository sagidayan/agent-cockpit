package tui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

var statusRank = map[state.Status]int{state.NeedsInput: 0, state.Working: 1, state.Background: 2, state.Done: 3, state.Idle: 4}

func rank(s state.Status) int {
	if r, ok := statusRank[s]; ok {
		return r
	}
	return len(statusRank)
}

var sorters = map[string]func(a, b agents.Live) int{
	"location": nil,
	"status":   func(a, b agents.Live) int { return cmp.Compare(rank(a.Status), rank(b.Status)) },
	"recent":   func(a, b agents.Live) int { return b.UpdatedAt.Compare(a.UpdatedAt) },
	"name":     func(a, b agents.Live) int { return cmp.Compare(strings.ToLower(name(a)), strings.ToLower(name(b))) },
	"cost":     func(a, b agents.Live) int { return cmp.Compare(b.Stats.CostUSD, a.Stats.CostUSD) },
}

func matches(a agents.Live, query string, extra ...string) bool {
	fields := append([]string{name(a), a.Session, a.Kind, a.Pane.Cwd, badgeFor(a.Status).label, a.Message, a.Prompt}, extra...)
	hay := strings.ToLower(strings.Join(fields, " "))
	for _, word := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(hay, word) {
			return false
		}
	}
	return true
}

func (m model) refreshView() model {
	prev, _ := m.selected()
	var visible []agents.Live
	for _, a := range m.all {
		if matches(a, m.filter, m.projectName(a)) {
			visible = append(visible, a)
		}
	}
	if by := sorters[m.sortBy]; by != nil {
		slices.SortStableFunc(visible, by)
	}
	slices.SortStableFunc(visible, func(a, b agents.Live) int {
		return cmp.Compare(m.favoriteRank(a), m.favoriteRank(b))
	})
	if m.grouped {
		byGroup(visible)
	}
	m = m.expandPending()
	m.agents, m.depth = treeOrder(visible, m.isExpanded)
	m.cursor = min(m.cursor, max(0, len(m.agents)-1))
	m = m.selectID(prev.ID)
	if m.pending != "" {
		m, m.pending = m.selectID(m.pending), ""
	}
	return m.showSelected()
}

func (m model) selectID(id string) model {
	for i, a := range m.agents {
		if a.ID == id {
			m.cursor = i
		}
	}
	return m
}

func (m model) isExpanded(id string) bool {
	return m.filter != "" || m.expanded[id]
}

func (m model) expandPending() model {
	for _, a := range m.all {
		if a.ID == m.pending && a.Parent != "" {
			m.expanded[a.Parent] = true
		}
	}
	return m
}

func (m model) toggleExpanded() model {
	a, ok := m.selected()
	if !ok {
		return m
	}
	id := a.ID
	if m.childCount(a) == 0 {
		if m.depth[id] == 0 {
			return m
		}
		id = a.Parent
	}
	m.expanded[id] = !m.expanded[id]
	return m.refreshView().selectID(id).showSelected()
}

func (m model) favoriteRank(a agents.Live) int {
	if m.isFavorite(a) {
		return 0
	}
	return 1
}

func (m model) projectName(a agents.Live) string {
	p, _ := projects.ForDir(m.projects, a.Pane.Cwd)
	return p.Name
}

func treeOrder(list []agents.Live, expanded func(id string) bool) ([]agents.Live, map[string]int) {
	present := make(map[string]bool, len(list))
	for _, a := range list {
		present[a.ID] = true
	}
	ordered := make([]agents.Live, 0, len(list))
	depth := map[string]int{}
	var walk func(a agents.Live, d int)
	walk = func(a agents.Live, d int) {
		ordered, depth[a.ID] = append(ordered, a), d
		if !expanded(a.ID) {
			return
		}
		for _, c := range list {
			if c.Parent == a.ID && c.ID != a.ID {
				walk(c, d+1)
			}
		}
	}
	for _, a := range list {
		if a.Parent == "" || !present[a.Parent] {
			walk(a, 0)
		}
	}
	return ordered, depth
}
