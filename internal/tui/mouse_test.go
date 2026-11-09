package tui

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestCardAt(t *testing.T) {
	t.Parallel()
	m := model{width: 40, height: 30}
	for i, dir := range []string{"/a", "/a-worktrees/x", "/b"} {
		id := fmt.Sprintf("%%%d", i)
		m.agents = append(m.agents, agents.Live{Pane: tmux.Pane{ID: id, WindowName: "w" + id, Cwd: dir}})
	}
	cases := []struct {
		name    string
		grouped bool
		y       int
		want    int
		wantOK  bool
	}{
		{"header", false, 0, 0, false},
		{"first card title", false, 3, 0, true},
		{"first card reply", false, 6, 0, true},
		{"gap", false, 7, 0, false},
		{"second card", false, 8, 1, true},
		{"third card", false, 13, 2, true},
		{"below cards", false, 17, 0, false},
		{"group header", true, 3, 0, false},
		{"grouped first card", true, 4, 0, true},
		{"same group has no header", true, 9, 1, true},
		{"next group header", true, 14, 0, false},
		{"next group card", true, 15, 2, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := m
			m.grouped = tc.grouped
			got, ok := m.cardAt(tc.y)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
