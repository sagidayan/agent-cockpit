package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestAge(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"seconds", 42 * time.Second, "42s"},
		{"minutes", 5*time.Minute + 30*time.Second, "5m"},
		{"hours", 3*time.Hour + 59*time.Minute, "3h"},
		{"days", 50 * time.Hour, "2d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := age(tc.d); got != tc.want {
				t.Errorf("age(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

func TestName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pane tmux.Pane
		want string
	}{
		{"label wins", tmux.Pane{Label: "billing bug", WindowName: "claude", Session: "work", Window: "2"}, "billing bug"},
		{"window name", tmux.Pane{WindowName: "claude", Session: "work", Window: "2"}, "claude"},
		{"location fallback", tmux.Pane{Session: "work", Window: "2"}, "work:2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := name(agents.Live{Pane: tc.pane}); got != tc.want {
				t.Errorf("name() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTLDR(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"first sentence", "Fixed the bug. Also renamed a helper.", "Fixed the bug."},
		{"skips heading and intro", "## Summary\nHere is what I did:\n- **Moved** the `card` code", "Moved the card code"},
		{"skips code fence", "```go\nx := 1\n```\nDone, tests pass", "Done, tests pass"},
		{"keeps dotted paths", "Edited ./internal/tui/view.go and view_test.go", "Edited ./internal/tui/view.go and view_test.go"},
		{"clips long text", strings.Repeat("word ", 30), strings.TrimSpace(strings.Repeat("word ", 18)) + "…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tldr(tc.in); got != tc.want {
				t.Errorf("tldr(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestHelpHints(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		agent agents.Live
		hint  string
		want  bool
	}{
		{"needs input", agents.Live{Agent: state.Agent{Status: state.NeedsInput}}, "p answer", true},
		{"working", agents.Live{Agent: state.Agent{Status: state.Working}}, "p answer", false},
		{"idle", agents.Live{Agent: state.Agent{Status: state.Idle}}, "p answer", false},
		{"claude with MCP down", agents.Live{Agent: state.Agent{Kind: "claude"}, MCPDown: true}, ":restart", true},
		{"MCP up", agents.Live{Agent: state.Agent{Kind: "claude"}}, "MCP down", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := model{width: 400, agents: []agents.Live{tc.agent}}
			if got := strings.Contains(ansi.Strip(m.help()), tc.hint); got != tc.want {
				t.Errorf("help() shows %q = %v, want %v", tc.hint, got, tc.want)
			}
		})
	}
}

func TestCardHead(t *testing.T) {
	t.Parallel()
	parent := agents.Live{Agent: state.Agent{Kind: "claude", ClosedCost: 0.5}, Pane: tmux.Pane{ID: "%1", WindowName: "lead"}, Stats: state.Stats{CostUSD: 1}, HasStats: true}
	child := agents.Live{Agent: state.Agent{Kind: "claude"}, Pane: tmux.Pane{ID: "%2", Parent: "%1", WindowName: "sub"}, Stats: state.Stats{CostUSD: 2}, HasStats: true}
	over := parent
	over.OverBudget = true
	stale := agents.Live{Agent: state.Agent{Kind: "claude"}, Pane: tmux.Pane{ID: "%3", WindowName: "solo"}, Stats: state.Stats{CostUSD: 9}}
	waiting := agents.Live{Agent: state.Agent{Kind: "claude", Status: state.NeedsInput}, Pane: tmux.Pane{ID: "%4", Parent: "%2", WindowName: "deep"}}
	long := over
	long.Label = strings.Repeat("long ", 60)
	cases := []struct {
		name     string
		all      []agents.Live
		expanded bool
		agent    agents.Live
		show     string
		hide     string
	}{
		{"parent shows tree total", []agents.Live{parent, child}, false, parent, "1.00 / tree 3.50", "over budget"},
		{"leaf shows own cost", []agents.Live{parent, child}, false, child, "2.00", "tree"},
		{"over budget root is flagged", []agents.Live{over, child}, false, over, "⚠ over budget", "MCP"},
		{"stale stats show no cost", []agents.Live{stale}, false, stale, "solo", "9.00"},
		{"collapsed parent rolls up its whole subtree", []agents.Live{parent, child, waiting}, false, parent, "lead [2] " + icons.status[state.NeedsInput] + " 1", "▾"},
		{"expanded parent counts its children", []agents.Live{parent, child, waiting}, true, parent, "lead [1]", icons.status[state.NeedsInput]},
		{"long name keeps the flags", []agents.Live{long, child}, false, long, "… [1] ⚠ over budget", "MCP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := model{width: 200, all: tc.all, depth: map[string]int{"%2": 1}, expanded: map[string]bool{"%1": tc.expanded}}
			got := ansi.Strip(strings.Join(m.cardHead(tc.agent, 190), "\n"))
			if !strings.Contains(got, tc.show) || strings.Contains(got, tc.hide) {
				t.Errorf("cardHead() = %q, want %q and not %q", got, tc.show, tc.hide)
			}
		})
	}
}

func TestGuides(t *testing.T) {
	t.Parallel()
	m := model{all: live("%1", "%1>%2", "%2>%3", "%2>%4", "%1>%5", "%6"), expanded: map[string]bool{"%1": true, "%2": true}}
	m = m.refreshView()
	cases := []struct {
		id   string
		head string
		body string
		gap  string
	}{
		{"%1", "", "", ""},
		{"%2", "├─", "│ ", " │"},
		{"%3", "│ ├─", "│ │ ", " │ │"},
		{"%4", "│ └─", "│   ", " │ │"},
		{"%5", "└─", "  ", " │"},
		{"%6", "", "", ""},
	}
	for i, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.id, m.agents[i].ID)
			head, body := m.guides(i)
			require.Equal(t, []string{tc.head, tc.body, tc.gap}, []string{head, body, ansi.Strip(m.gapGuide(i))})
		})
	}
}

func TestUpdateHelp(t *testing.T) {
	t.Parallel()
	m := model{width: 80, height: 30}.openHelp()
	bottom := len(m.helpRows()) - m.listHeight()
	cases := []struct {
		name        string
		keys        []rune
		wantTop     int
		wantOverlay string
	}{
		{"scroll down and up", []rune{'j', 'j', 'k'}, 1, "help"},
		{"not above the top", []rune{'k'}, 0, "help"},
		{"pages stop at the bottom", []rune{tea.KeyPgDown, tea.KeyPgDown, tea.KeyPgDown}, bottom, "help"},
		{"esc closes", []rune{'j', tea.KeyEscape}, 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := m
			for _, k := range tc.keys {
				next, _ := m.updateHelp(tea.KeyPressMsg{Code: k})
				m = next.(model)
			}
			require.Equal(t, tc.wantTop, m.helpTop)
			require.Equal(t, tc.wantOverlay, m.overlay)
		})
	}
}
