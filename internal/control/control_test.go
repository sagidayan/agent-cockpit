package control

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestMatch(t *testing.T) {
	t.Parallel()
	live := []agents.Live{
		{Pane: tmux.Pane{ID: "%1", WindowName: "claude", Label: "billing bug"}},
		{Pane: tmux.Pane{ID: "%2", WindowName: "claude"}},
		{Pane: tmux.Pane{ID: "%3", WindowName: "web"}},
	}
	cases := []struct {
		name    string
		target  string
		want    string
		wantErr string
	}{
		{"pane id", "%2", "%2", ""},
		{"label case insensitive", "Billing Bug", "%1", ""},
		{"window name", "web", "%3", ""},
		{"ambiguous window name", "claude", "", "matches 2 agents"},
		{"no match", "nope", "", "no agent matches"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := match(live, tc.target)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, a.ID)
		})
	}
}
