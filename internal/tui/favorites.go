package tui

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/favorites"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

func (m model) isFavorite(a agents.Live) bool {
	return favorites.Index(m.favorites, a) >= 0
}

func (m model) toggleFavorite() (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		added, err := favorites.Toggle(a, m.all)
		if m.actionErr = err; err != nil {
			return m, nil
		}
		m.notice = "✓ removed " + name(a) + " from favorites"
		if added {
			m.notice = "✓ favorited " + name(a)
		}
		return m, load
	})
}

func (m model) openFavorites() model {
	m.overlay, m.favCursor = "favorites", 0
	return m
}

func (m model) updateFavorites(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	n := len(m.favorites)
	switch msg.String() {
	case "j", "down":
		m.favCursor = min(m.favCursor+1, max(0, n-1))
	case "k", "up":
		m.favCursor = max(m.favCursor-1, 0)
	case "enter", "right", "l":
		if n > 0 {
			return m.openFavorite(m.favorites[m.favCursor])
		}
	case "d", "x":
		if n > 0 {
			m.actionErr = favorites.Remove(m.favorites[m.favCursor])
			m.favorites = slices.Delete(slices.Clone(m.favorites), m.favCursor, m.favCursor+1)
			m.favCursor = min(m.favCursor, max(0, len(m.favorites)-1))
		}
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.overlay = ""
	}
	return m, nil
}

func (m model) openFavorite(e snapshot.Entry) (tea.Model, tea.Cmd) {
	m.overlay = ""
	for i, a := range m.agents {
		if snapshot.Matches(e, a) {
			m.cursor = i
			return m.showSelected(), nil
		}
	}
	if m.view == nil {
		m.actionErr = fmt.Errorf("run cockpit inside tmux to resume sessions")
		return m, nil
	}
	return m, restore([]snapshot.Entry{e})
}

func (m model) favoritesView() string {
	rows := []string{" " + titleStyle.Render(withIcon(icons.star, "favorites")) + mutedStyle.Render("  ⏎ open/resume · d remove · esc close")}
	if len(m.favorites) == 0 {
		rows = append(rows, "", mutedStyle.Render("   No favorites yet. Press f on an agent."))
	}
	for i, e := range m.favorites {
		rows = append(rows, m.favoriteRow(i, e))
	}
	for i, r := range rows {
		rows[i] = ansi.Truncate(r, m.width, "…")
	}
	return strings.Join(rows, "\n")
}

func (m model) favoriteRow(i int, e snapshot.Entry) string {
	title := withIcon(icons.kinds[e.Kind], cmp.Or(e.Label, e.Name, filepath.Base(e.Dir)))
	if n := len(e.Children); n > 0 {
		title += fmt.Sprintf(" +%d", n)
	}
	status := subtleStyle.Render(withIcon(icons.restore, "resume"))
	if favorites.Running(e, m.all) {
		status = badges[state.Done].color.Render("running")
	}
	cursor, style := "   ", mutedStyle
	if i == m.favCursor {
		cursor, style = " "+barStyle.Render("┃")+" ", nameStyle
	}
	return cursor + style.Render(title) + "  " + subtleStyle.Render(paths.Contract(e.Dir)) + "  " + status
}
