package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

func TestResumeClosed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name    string
		closed  state.Closed
		wantErr string
	}{
		{"no session", state.Closed{Name: "w", Dir: dir}, "w has no session to resume"},
		{"folder gone", state.Closed{SessionID: "s1", Dir: dir + "/gone"}, "no longer exists"},
		{"outside tmux", state.Closed{SessionID: "s1", Dir: dir}, "run cockpit inside tmux"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			next, cmd := model{overlay: "closed"}.resumeClosed(tc.closed)
			m := next.(model)
			require.Nil(t, cmd)
			require.Empty(t, m.overlay)
			require.ErrorContains(t, m.actionErr, tc.wantErr)
		})
	}
}
