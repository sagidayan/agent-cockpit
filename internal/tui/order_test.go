package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func live(ids ...string) []agents.Live {
	var list []agents.Live
	for _, id := range ids {
		parent, child, ok := strings.Cut(id, ">")
		if !ok {
			parent, child = "", id
		}
		list = append(list, agents.Live{Pane: tmux.Pane{ID: child, Parent: parent}, SessionID: "s" + child})
	}
	return list
}

func ids(list []agents.Live) []string {
	var out []string
	for _, a := range list {
		out = append(out, a.ID)
	}
	return out
}

func TestArrange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		order []string
		live  []agents.Live
		want  []string
	}{
		{"first load keeps the given order", nil, live("%2", "%1"), []string{"%2", "%1"}},
		{"new agents join at the bottom", []string{"%3", "%1"}, live("%1", "%2", "%3"), []string{"%3", "%1", "%2"}},
		{"closed agents drop out", []string{"%3", "%1", "%2"}, live("%1", "%2"), []string{"%1", "%2"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, arrange(tc.order, tc.live))
		})
	}
}

func TestShift(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		all       []agents.Live
		favorites []snapshot.Entry
		sortBy    string
		selected  string
		step      int
		want      []string
		wantSaved bool
	}{
		{"down", live("%1", "%2", "%3"), nil, "", "%1", 1, []string{"%2", "%1", "%3"}, true},
		{"up", live("%1", "%2", "%3"), nil, "", "%3", -1, []string{"%1", "%3", "%2"}, true},
		{"top stays", live("%1", "%2"), nil, "", "%1", -1, []string{"%1", "%2"}, false},
		{"jumps over children", live("%1", "%1>%2", "%3"), nil, "", "%1", 1, []string{"%3", "%1", "%2"}, true},
		{"child stays with its parent", live("%1", "%1>%2", "%3"), nil, "", "%2", 1, []string{"%1", "%2", "%3"}, false},
		{"not past a favorite", live("%1", "%2"), []snapshot.Entry{{SessionID: "s%1"}}, "", "%2", -1, []string{"%1", "%2"}, false},
		{"not while sorted", live("%1", "%2"), nil, "name", "%1", 1, []string{"%1", "%2"}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := model{all: tc.all, favorites: tc.favorites, sortBy: tc.sortBy, order: arrange(nil, tc.all), expanded: map[string]bool{"%1": true}}
			m = m.refreshView().selectID(tc.selected)
			m, cmd := m.shift(tc.step)
			require.Equal(t, tc.want, ids(m.agents))
			require.Equal(t, tc.wantSaved, cmd != nil)
			require.Equal(t, tc.selected, ids(m.agents)[m.cursor])
		})
	}
}

func TestListJumps(t *testing.T) {
	t.Parallel()
	press := func(m model, keys string) model {
		for _, k := range keys {
			next, _ := m.updateList(tea.KeyPressMsg{Code: k, Text: string(k)})
			m = next.(model)
		}
		return m
	}
	cases := []struct {
		keys string
		from int
		want int
	}{
		{"G", 0, 2},
		{"gg", 2, 0},
		{"gj", 0, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.keys, func(t *testing.T) {
			t.Parallel()
			m := model{all: live("%1", "%2", "%3"), expanded: map[string]bool{}}
			m = m.refreshView()
			m.cursor = tc.from
			m = press(m, tc.keys)
			require.Equal(t, tc.want, m.cursor)
			require.Empty(t, m.chord)
		})
	}
}
