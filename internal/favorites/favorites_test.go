package favorites

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestIndexAndRunning(t *testing.T) {
	t.Parallel()
	list := []snapshot.Entry{
		{Kind: "claude", Dir: "/a", SessionID: "s1"},
		{Kind: "opencode", Dir: "/b", Label: "web"},
	}
	agent := func(kind, dir, label, session string) agents.Live {
		return agents.Live{Agent: state.Agent{Kind: kind, SessionID: session}, Pane: tmux.Pane{Cwd: dir, Label: label}}
	}
	cases := []struct {
		name  string
		agent agents.Live
		want  int
	}{
		{"same session id", agent("claude", "/elsewhere", "", "s1"), 0},
		{"different session id", agent("claude", "/a", "", "s2"), -1},
		{"kind dir and label", agent("opencode", "/b", "web", ""), 1},
		{"label differs", agent("opencode", "/b", "api", ""), -1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Index(list, tc.agent))
			if tc.want >= 0 {
				require.True(t, Running(list[tc.want], []agents.Live{tc.agent}))
			}
		})
	}
}

func TestSyncKeepsSessionID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	fav := snapshot.Entry{Kind: "claude", Dir: "/a", Name: "fav", SessionID: "s1"}
	require.NoError(t, save([]snapshot.Entry{fav}))

	fresh := agents.Live{Agent: state.Agent{Kind: "claude"}, Pane: tmux.Pane{ID: "%9", Cwd: "/a"}}
	require.NoError(t, Sync([]agents.Live{fresh}))

	got, err := Load()
	require.NoError(t, err)
	require.Equal(t, []snapshot.Entry{fav}, got)
}
