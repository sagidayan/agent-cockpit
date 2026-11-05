package agents

import (
	"testing"

	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
	"github.com/stretchr/testify/require"
)

func TestJoin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		panes     []tmux.Pane
		stored    []state.Agent
		wantKinds map[string]string
		wantStale []string
	}{
		{
			"tracked pane",
			[]tmux.Pane{{ID: "%1", Command: "node"}},
			[]state.Agent{{Pane: "%1", Kind: "opencode", Status: state.Done, Server: "s1"}},
			map[string]string{"%1": "opencode"},
			nil,
		},
		{
			"state from another tmux server is stale",
			[]tmux.Pane{{ID: "%0", Command: "cockpit"}},
			[]state.Agent{{Pane: "%0", Kind: "claude", Server: "old"}},
			map[string]string{},
			[]string{"%0"},
		},
		{
			"other server state on agent pane is rediscovered",
			[]tmux.Pane{{ID: "%0", Command: "claude"}},
			[]state.Agent{{Pane: "%0", Kind: "claude", SessionID: "old", Server: "old"}},
			map[string]string{"%0": "claude"},
			[]string{"%0"},
		},
		{
			"legacy state kept on agent pane",
			[]tmux.Pane{{ID: "%1", Command: "claude"}},
			[]state.Agent{{Pane: "%1", Kind: "claude", Status: state.Done}},
			map[string]string{"%1": "claude"},
			nil,
		},
		{
			"legacy state on non-agent pane is stale",
			[]tmux.Pane{{ID: "%1", Command: "zsh"}},
			[]state.Agent{{Pane: "%1", Kind: "claude"}},
			map[string]string{},
			[]string{"%1"},
		},
		{
			"untracked agent discovered",
			[]tmux.Pane{{ID: "%1", Command: "claude", Cwd: "/repo"}, {ID: "%2", Command: "codex"}},
			nil,
			map[string]string{"%1": "claude", "%2": "codex"},
			nil,
		},
		{
			"non-agent pane ignored",
			[]tmux.Pane{{ID: "%1", Command: "zsh"}},
			nil,
			map[string]string{},
			nil,
		},
		{
			"dead pane is stale",
			[]tmux.Pane{{ID: "%1", Command: "zsh"}},
			[]state.Agent{{Pane: "%9", Kind: "claude"}},
			map[string]string{},
			[]string{"%9"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			live, stale := join(tc.panes, tc.stored, "s1")
			kinds := map[string]string{}
			for _, l := range live {
				require.Equal(t, l.ID, l.Agent.Pane)
				kinds[l.ID] = l.Kind
			}
			require.Equal(t, tc.wantKinds, kinds)
			require.Equal(t, tc.wantStale, stale)
		})
	}
}
