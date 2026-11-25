package tui

import (
	"os"
	"path/filepath"
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

func TestCompletions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, d := range []string{"api", "app", "web"} {
		require.NoError(t, os.Mkdir(filepath.Join(root, d), 0o755))
	}
	cases := []struct {
		name  string
		input string
		want  []string
		tab   string
	}{
		{name: "command names", input: "rest", want: []string{"restart ", "restore "}, tab: "restart "},
		{name: "common command prefix", input: "res", want: []string{"restart ", "restore "}, tab: "rest"},
		{name: "project add dir", input: "project add api " + root + "/a", want: []string{"project add api " + root + "/api/", "project add api " + root + "/app/"}, tab: "project add api " + root + "/ap"},
		{name: "project add unique dir", input: "project add web " + root + "/w", want: []string{"project add web " + root + "/web/"}, tab: "project add web " + root + "/web/"},
		{name: "project add name is not a path", input: "project add " + root + "/w", tab: "project add " + root + "/w"},
		{name: "new dir", input: "new " + root + "/w", want: []string{"new " + root + "/web/"}, tab: "new " + root + "/web/"},
		{name: "new dir after agent", input: "new claude " + root + "/w", want: []string{"new claude " + root + "/web/"}, tab: "new claude " + root + "/web/"},
		{name: "command abbreviation", input: "proj add x " + root + "/w", want: []string{"proj add x " + root + "/web/"}, tab: "proj add x " + root + "/web/"},
		{name: "no path argument", input: "send " + root + "/w", tab: "send " + root + "/w"},
		{name: "shell command", input: "!ls " + root + "/w", tab: "!ls " + root + "/w"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			found := completions(tc.input)
			var values []string
			for _, f := range found {
				values = append(values, f.value)
			}
			require.Equal(t, tc.want, values)
			require.Equal(t, tc.tab, completeLine(tc.input, found))
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
		{
			"grouped children keep their order across folders",
			inDirs([]agents.Live{agent("%2", ""), agent("%4", "%2"), agent("%3", "%2")}, "/a", "/b", "/a"),
			nil,
			true,
			"%4",
			[]string{"%2", "%4", "%3"},
			1,
		},
	}
	for _, tc := range cases {
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
