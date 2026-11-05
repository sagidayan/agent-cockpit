package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestClosable(t *testing.T) {
	t.Parallel()
	agent := func(id, parent string) agents.Live {
		return agents.Live{Pane: tmux.Pane{ID: id, Parent: parent}}
	}
	live := []agents.Live{agent("%1", ""), agent("%2", "%1"), agent("%3", "%2"), agent("%4", "%1"), agent("%5", "")}
	cases := []struct {
		name    string
		pane    string
		caller  string
		want    []string
		wantErr string
	}{
		{"child without children", "%4", "%1", []string{"%4"}, ""},
		{"sub-master closes its children first", "%2", "%1", []string{"%3", "%2"}, ""},
		{"grandchild is not mine", "%3", "%1", nil, "not your child"},
		{"unrelated agent", "%5", "%1", nil, "not your child"},
		{"no caller pane", "%2", "", nil, "not your child"},
		{"missing pane", "%9", "%1", nil, "no agent in pane %9"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := closable(live, tc.pane, tc.caller)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			var ids []string
			for _, a := range got {
				ids = append(ids, a.ID)
			}
			require.Equal(t, tc.want, ids)
		})
	}
}
