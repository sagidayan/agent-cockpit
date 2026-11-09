package tui

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/worktree"
)

func groupDir(a agents.Live) string {
	return worktree.Origin(a.Pane.Cwd)
}

func byGroup(list []agents.Live) {
	var seen []string
	for _, a := range list {
		if g := groupDir(a); !slices.Contains(seen, g) {
			seen = append(seen, g)
		}
	}
	slices.SortStableFunc(list, func(a, b agents.Live) int {
		return cmp.Compare(slices.Index(seen, groupDir(a)), slices.Index(seen, groupDir(b)))
	})
}

func (m model) startsGroup(i int) bool {
	a := m.agents[i]
	if !m.grouped || m.depth[a.ID] > 0 {
		return false
	}
	j := i - 1
	for j >= 0 && m.depth[m.agents[j].ID] > 0 {
		j--
	}
	return j < 0 || groupDir(m.agents[j]) != groupDir(a)
}

func (m model) groupHeader(a agents.Live) string {
	dir := groupDir(a)
	title := withIcon(icons.folder, filepath.Base(dir))
	if p, ok := projects.ForDir(m.projects, dir); ok {
		title = withIcon(icons.project, p.Name)
	}
	title = " " + titleStyle.Render(title) + " "
	return title + subtleStyle.Render(strings.Repeat("─", max(0, m.width-lipgloss.Width(title)-1)))
}

func cmdGroup(m model, _ string) (model, tea.Cmd) {
	m.grouped = !m.grouped
	m.notice = "✓ grouped by project"
	if !m.grouped {
		m.notice = "✓ ungrouped"
	}
	grouped := m.grouped
	return m.refreshView(), func() tea.Msg { return actionMsg{err: state.SaveGrouped(grouped)} }
}
