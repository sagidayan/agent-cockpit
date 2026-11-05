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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Missing([]Entry{tc.saved}, tc.live)
			require.Equal(t, tc.want, len(got) == 1)
		})
	}
}

func TestSaveLoadClear(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	got, err := Load()
	require.NoError(t, err)
	require.Nil(t, got)

	require.NoError(t, Save(FromLive([]agents.Live{live("claude", "/a", "bug", "s1")})))
	got, err = Load()
	require.NoError(t, err)
	require.Equal(t, []Entry{{Kind: "claude", Dir: "/a", Label: "bug", SessionID: "s1"}}, got)

	require.NoError(t, Clear())
	require.NoError(t, Clear())
	got, err = Load()
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestFromLiveTree(t *testing.T) {
	t.Parallel()
	agent := func(id, parent, mode string) agents.Live {
		return agents.Live{Agent: state.Agent{Kind: "claude", SessionID: "s" + id}, Pane: tmux.Pane{ID: id, Parent: parent, Mode: mode}}
	}
	live := []agents.Live{
		agent("%1", "", "master"),
		agent("%2", "%1", ""),
		agent("%3", "%2", ""),
		agent("%4", "", ""),
		agent("%5", "%9", ""),
	}
	got := FromLive(live)
	require.Len(t, got, 3)
	require.Equal(t, "master", got[0].Mode)
	require.Equal(t, "s%2", got[0].Children[0].SessionID)
	require.Equal(t, "s%3", got[0].Children[0].Children[0].SessionID)
	require.Empty(t, got[1].Children)
	require.Equal(t, "s%5", got[2].SessionID)
}
