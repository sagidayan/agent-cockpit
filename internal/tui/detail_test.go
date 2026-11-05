package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
	"github.com/sagidayan/agent-cockpit/internal/transcript"
)

func conversation(n int) []transcript.Message {
	msgs := make([]transcript.Message, n)
	for i := range msgs {
		msgs[i] = transcript.Message{Role: transcript.Assistant, Text: "line"}
	}
	return msgs
}

func detailModel() model {
	m := model{width: 60, height: 30}
	m.agents = []agents.Live{{Pane: tmux.Pane{ID: "%1"}}, {Pane: tmux.Pane{ID: "%2"}}}
	m.detail = detail{open: true, top: 10, id: "%1", conv: conversation(100)}
	return m
}

func TestSetDetail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		cursor   int
		msg      detailMsg
		wantTop  int
		wantText string
	}{
		{"reload keeps scroll", 0, detailMsg{id: "%1", conv: conversation(120)}, 10, "11-"},
		{"other agent ignored", 0, detailMsg{id: "%2", screen: "$ ls", fromPane: true}, 10, "of 107"},
		{"new selection resets scroll", 1, detailMsg{id: "%2", screen: "$ ls", fromPane: true}, 0, "$ ls"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := detailModel()
			m.cursor = tc.cursor
			got := m.setDetail(tc.msg)
			require.Equal(t, tc.wantTop, got.detail.top)
			require.Contains(t, got.detailView(), tc.wantText)
		})
	}
}

func TestScrollDetail(t *testing.T) {
	t.Parallel()
	m := detailModel()
	maxTop := m.detailMaxTop()
	cases := []struct {
		name string
		n    int
		want int
	}{
		{"down", 1, 11},
		{"up", -1, 9},
		{"clamps at top", -50, 0},
		{"clamps at bottom", 500, maxTop},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, m.scrollDetail(tc.n).detail.top)
		})
	}
}

func TestDetailViewFitsHeight(t *testing.T) {
	t.Parallel()
	m := detailModel()
	m.agents[0].Prompt = strings.Repeat("word ", 40)
	view := m.detailView()
	require.Equal(t, m.listHeight(), strings.Count(view, "\n")+1)
	require.Contains(t, view, "11-")
}
