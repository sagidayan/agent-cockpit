package tui

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestLookup(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input   string
		want    string
		wantErr string
	}{
		{"quit", "quit", ""},
		{"q", "quit", ""},
		{"b", "broadcast", ""},
		{"sh", "shell", ""},
		{"clo", "closed", ""},
		{"cle", "clean", ""},
		{"cl", "", "ambiguous"},
		{"gr", "group", ""},
		{"rest", "", "ambiguous"},
		{"resta", "restart", ""},
		{"s", "", "ambiguous"},
		{"nope", "", "unknown"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			c, err := lookup(tc.input)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, c.name)
		})
	}
}

func TestMatches(t *testing.T) {
	t.Parallel()
	a := agents.Live{
		Agent: state.Agent{Kind: "claude", Status: state.NeedsInput, Message: "Needs permission to run Bash"},
		Pane:  tmux.Pane{WindowName: "payments", Session: "work", Cwd: "/src/payments-service"},
	}
	cases := []struct {
		name  string
		query string
		want  bool
	}{
		{"empty", "", true},
		{"name", "pay", true},
		{"case insensitive", "PAYMENTS", true},
		{"status label", "needs input", true},
		{"message", "bash", true},
		{"all words required", "claude codex", false},
		{"no match", "frontend", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, matches(a, tc.query))
		})
	}
}

func TestTreeOrder(t *testing.T) {
	t.Parallel()
	agent := func(id, parent string) agents.Live {
		return agents.Live{Pane: tmux.Pane{ID: id, Parent: parent}}
	}
	cases := []struct {
		name      string
		in        []agents.Live
		collapsed []string
		wantOrder []string
		wantDepth map[string]int
	}{
		{"flat", []agents.Live{agent("%1", ""), agent("%2", "")}, nil, []string{"%1", "%2"}, map[string]int{"%1": 0, "%2": 0}},
		{
			"children follow parent",
			[]agents.Live{agent("%3", "%1"), agent("%2", ""), agent("%1", ""), agent("%4", "%3")},
			nil,
			[]string{"%2", "%1", "%3", "%4"},
			map[string]int{"%1": 0, "%2": 0, "%3": 1, "%4": 2},
		},
		{"orphan is a root", []agents.Live{agent("%5", "%9")}, nil, []string{"%5"}, map[string]int{"%5": 0}},
		{
			"collapsed parent hides children",
			[]agents.Live{agent("%1", ""), agent("%2", "%1"), agent("%3", "")},
			[]string{"%1"},
			[]string{"%1", "%3"},
			map[string]int{"%1": 0, "%3": 0},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, depth := treeOrder(tc.in, func(id string) bool { return !slices.Contains(tc.collapsed, id) })
			var ids []string
			for _, a := range got {
				ids = append(ids, a.ID)
			}
			require.Equal(t, tc.wantOrder, ids)
			require.Equal(t, tc.wantDepth, depth)
		})
	}
}

func TestRefreshViewOrder(t *testing.T) {
	t.Parallel()
	agent := func(id, parent string) agents.Live {
		return agents.Live{Pane: tmux.Pane{ID: id, Parent: parent}, SessionID: "s" + id}
	}
	fav := func(id string) snapshot.Entry { return snapshot.Entry{SessionID: "s" + id} }
	cases := []struct {
		name       string
		all        []agents.Live
		favorites  []snapshot.Entry
		grouped    bool
		selected   string
		wantOrder  []string
		wantCursor int
	}{
		{"no favorites", []agents.Live{agent("%1", ""), agent("%2", "")}, nil, false, "%1", []string{"%1", "%2"}, 0},
		{"favorite moves up", []agents.Live{agent("%1", ""), agent("%2", "")}, []snapshot.Entry{fav("%2")}, false, "%2", []string{"%2", "%1"}, 0},
		{
			"favorite parent brings children",
			[]agents.Live{agent("%1", ""), agent("%2", ""), agent("%3", "%2")},
			[]snapshot.Entry{fav("%2")},
			false,
			"%3",
			[]string{"%2", "%3", "%1"},
			1,
		},
		{
			"favorite child stays under parent",
			[]agents.Live{agent("%1", ""), agent("%2", ""), agent("%3", "%2")},
			[]snapshot.Entry{fav("%3")},
			false,
			"%1",
			[]string{"%1", "%2", "%3"},
			0,
		},
		{
			"grouped by project in first-seen order",
			inDirs([]agents.Live{agent("%1", ""), agent("%2", ""), agent("%3", ""), agent("%4", "")}, "/a", "/b", "/a-worktrees/x", "/b"),
			nil,
			true,
			"%2",
			[]string{"%1", "%3", "%2", "%4"},
			2,
		},
		{
			"group with a favorite goes first",
			inDirs([]agents.Live{agent("%1", ""), agent("%2", ""), agent("%3", "")}, "/a", "/b", "/a"),
			[]snapshot.Entry{fav("%2")},
			true,
			"%1",
			[]string{"%2", "%1", "%3"},
			1,
		},
		{
			"grouped child stays under its parent",
			inDirs([]agents.Live{agent("%1", ""), agent("%2", ""), agent("%3", "%2")}, "/a", "/b", "/a"),
			nil,
			true,
			"%3",
			[]string{"%1", "%2", "%3"},
			2,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := model{all: tc.all, favorites: tc.favorites, grouped: tc.grouped, agents: tc.all, expanded: map[string]bool{"%2": true}}
			m.cursor = slices.IndexFunc(tc.all, func(a agents.Live) bool { return a.ID == tc.selected })
			m = m.refreshView()
			var ids []string
			for _, a := range m.agents {
				ids = append(ids, a.ID)
			}
			require.Equal(t, tc.wantOrder, ids)
			require.Equal(t, tc.wantCursor, m.cursor)
		})
	}
}

func TestFamily(t *testing.T) {
	t.Parallel()
	agent := func(id, parent string) agents.Live {
		return agents.Live{Pane: tmux.Pane{ID: id, Parent: parent}}
	}
	all := []agents.Live{agent("%1", ""), agent("%2", "%1"), agent("%3", "%2"), agent("%4", ""), agent("%5", "%5")}
	cases := []struct {
		name string
		id   string
		want []string
	}{
		{"master with grandchildren", "%1", []string{"%1", "%2", "%3"}},
		{"child brings its own children", "%2", []string{"%2", "%3"}},
		{"no children", "%4", []string{"%4"}},
		{"self parent ignored", "%5", []string{"%5"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := model{all: all}
			var ids []string
			for _, a := range m.family(all[slices.IndexFunc(all, func(a agents.Live) bool { return a.ID == tc.id })]) {
				ids = append(ids, a.ID)
			}
			require.Equal(t, tc.want, ids)
		})
	}
}
