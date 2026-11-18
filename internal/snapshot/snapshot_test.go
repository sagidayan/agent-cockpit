package snapshot

import (
	"testing"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
	"github.com/stretchr/testify/require"
)

func live(kind, dir, label, session string) agents.Live {
	return agents.Live{
		Agent: state.Agent{Kind: kind, SessionID: session},
		Pane:  tmux.Pane{Cwd: dir, Label: label},
	}
}

func TestMissing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		saved Entry
		live  []agents.Live
		want  bool
	}{
		{"nothing running", Entry{Kind: "claude", Dir: "/a"}, nil, true},
		{"same session", Entry{Kind: "claude", Dir: "/a", SessionID: "s1"}, []agents.Live{live("claude", "/b", "", "s1")}, false},
		{"different session same dir", Entry{Kind: "claude", Dir: "/a", SessionID: "s1"}, []agents.Live{live("claude", "/a", "", "s2")}, true},
		{"saved session live without", Entry{Kind: "codex", Dir: "/a", SessionID: "s1"}, []agents.Live{live("codex", "/a", "", "")}, true},
		{"kind and dir", Entry{Kind: "opencode", Dir: "/a"}, []agents.Live{live("opencode", "/a", "x", "s2")}, false},
		{"other kind", Entry{Kind: "claude", Dir: "/a"}, []agents.Live{live("codex", "/a", "", "")}, true},
		{"other dir", Entry{Kind: "claude", Dir: "/a"}, []agents.Live{live("claude", "/b", "", "")}, true},
		{"same label", Entry{Kind: "claude", Dir: "/a", Label: "bug"}, []agents.Live{live("claude", "/a", "bug", "")}, false},
		{"other label", Entry{Kind: "claude", Dir: "/a", Label: "bug"}, []agents.Live{live("claude", "/a", "", "")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Missing([]Entry{tc.saved}, tc.live)
			require.Equal(t, tc.want, len(got) == 1)
		})
	}
}

func TestPersistence(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	got, err := Load()
	require.NoError(t, err)
	require.Nil(t, got)

	require.NoError(t, Save(Merge(nil, []agents.Live{live("claude", "/a", "bug", "s1")})))
	got, err = Load()
	require.NoError(t, err)
	require.Equal(t, []Entry{{Kind: "claude", Dir: "/a", Label: "bug", SessionID: "s1"}}, got)

	require.NoError(t, Save([]Entry{node("1", node("2")), node("3")}))
	require.NoError(t, Forget(pane("1", "")))
	require.NoError(t, Dismiss([]Entry{node("3")}))
	got, err = Load()
	require.NoError(t, err)
	require.Equal(t, []Entry{node("2")}, got)

	require.NoError(t, Clear())
	require.NoError(t, Clear())
	got, err = Load()
	require.NoError(t, err)
	require.Nil(t, got)
}

func pane(id, parent string) agents.Live {
	return agents.Live{Agent: state.Agent{Kind: "claude", SessionID: "s" + id}, Pane: tmux.Pane{ID: id, Parent: parent, Cwd: "/" + id}}
}

func node(id string, children ...Entry) Entry {
	return Entry{Kind: "claude", Dir: "/" + id, SessionID: "s" + id, Children: children}
}

func TestMerge(t *testing.T) {
	t.Parallel()
	relabeled := pane("1", "")
	relabeled.Label, relabeled.Mode, relabeled.Paused = "new", "master", true
	paused := node("1")
	paused.Paused = true
	cases := []struct {
		name  string
		saved []Entry
		live  []agents.Live
		want  []Entry
	}{
		{"nothing", nil, nil, nil},
		{"live trees", nil, []agents.Live{pane("1", ""), pane("2", "1"), pane("3", "2"), pane("4", ""), pane("5", "9")}, []Entry{node("1", node("2", node("3"))), node("4"), node("5")}},
		{"new agent appended", []Entry{node("1")}, []agents.Live{pane("1", ""), pane("2", "")}, []Entry{node("1"), node("2")}},
		{"vanished agent kept", []Entry{node("1"), node("2")}, []agents.Live{pane("2", "")}, []Entry{node("1"), node("2")}},
		{"vanished child kept under live parent", []Entry{node("1", node("2"))}, []agents.Live{pane("1", "")}, []Entry{node("1", node("2"))}},
		{"vanished grandchild kept", []Entry{node("1", node("2", node("3")))}, []agents.Live{pane("1", ""), pane("2", "1")}, []Entry{node("1", node("2", node("3")))}},
		{"live children before vanished ones", []Entry{node("1", node("2"), node("3"))}, []agents.Live{pane("1", ""), pane("3", "1")}, []Entry{node("1", node("3"), node("2"))}},
		{"live child stays under vanished parent", []Entry{node("1", node("2"))}, []agents.Live{pane("2", "1")}, []Entry{node("1", node("2"))}},
		{"live state wins", []Entry{node("1")}, []agents.Live{relabeled}, []Entry{{Kind: "claude", Dir: "/1", Label: "new", SessionID: "s1", Mode: "master", Paused: true}}},
		{"vanished keeps paused", []Entry{paused}, nil, []Entry{paused}},
		{"matched without session", []Entry{{Kind: "claude", Dir: "/1"}}, []agents.Live{pane("1", "")}, []Entry{node("1")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Merge(tc.saved, tc.live))
		})
	}
}

func TestWithout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		saved  []Entry
		killed string
		want   []Entry
	}{
		{"root", []Entry{node("1"), node("2")}, "1", []Entry{node("2")}},
		{"children promoted", []Entry{node("1", node("2", node("3")))}, "1", []Entry{node("2", node("3"))}},
		{"nested", []Entry{node("1", node("2", node("3")))}, "2", []Entry{node("1", node("3"))}},
		{"not saved", []Entry{node("1")}, "2", []Entry{node("1")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, without(tc.saved, pane(tc.killed, "")))
		})
	}
}
