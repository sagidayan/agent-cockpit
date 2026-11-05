package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestCardAt(t *testing.T) {
	t.Parallel()
	m := model{width: 40, height: 30}
	for _, id := range []string{"%0", "%1", "%2"} {
		m.agents = append(m.agents, agents.Live{Pane: tmux.Pane{ID: id, WindowName: "w" + id}})
	}
	cases := []struct {
		name   string
		y      int
		want   int
		wantOK bool
	}{
		{"header", 0, 0, false},
		{"first card title", 3, 0, true},
		{"first card reply", 6, 0, true},
		{"gap", 7, 0, false},
		{"second card", 8, 1, true},
		{"third card", 13, 2, true},
		{"below cards", 17, 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := m.cardAt(tc.y)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
