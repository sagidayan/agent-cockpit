package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestPausePlan(t *testing.T) {
	t.Parallel()
	agent := func(id string, s state.Status) agents.Live {
		return agents.Live{Agent: state.Agent{Status: s}, Pane: tmux.Pane{ID: id}}
	}
	cases := []struct {
		name       string
		scope      []agents.Live
		pauseFirst bool
		wantResume bool
		want       []string
	}{
		{"pause the working ones", []agents.Live{agent("%1", state.Working), agent("%2", state.Idle), agent("%3", state.Working)}, false, false, []string{"%1", "%3"}},
		{"resume a paused tree", []agents.Live{agent("%1", state.Paused), agent("%2", state.Working)}, false, true, []string{"%1"}},
		{"pause all comes before resume", []agents.Live{agent("%1", state.Paused), agent("%2", state.Working)}, true, false, []string{"%2"}},
		{"resume all once nothing works", []agents.Live{agent("%1", state.Paused), agent("%2", state.Done)}, true, true, []string{"%1"}},
		{"background and waiting agents are left alone", []agents.Live{agent("%1", state.Background), agent("%2", state.NeedsInput)}, false, false, nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resume, targets := pausePlan(tc.scope, tc.pauseFirst)
			require.Equal(t, tc.wantResume, resume)
			require.Equal(t, tc.want, ids(targets))
		})
	}
}
