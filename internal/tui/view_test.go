package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

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
		tc := tc
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
		tc := tc
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
		tc := tc
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
		tc := tc
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
	cases := []struct {
		name  string
		all   []agents.Live
		agent agents.Live
		show  string
		hide  string
	}{
		{"parent shows tree total", []agents.Live{parent, child}, parent, "1.00 / tree 3.50", "over budget"},
		{"leaf shows own cost", []agents.Live{parent, child}, child, "2.00", "tree"},
		{"over budget root is flagged", []agents.Live{over, child}, over, "⚠ over budget", "MCP"},
		{"stale stats show no cost", []agents.Live{stale}, stale, "solo", "9.00"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := model{width: 200, all: tc.all, depth: map[string]int{"%2": 1}}
			got := ansi.Strip(strings.Join(m.cardHead(tc.agent, 190), "\n"))
			if !strings.Contains(got, tc.show) || strings.Contains(got, tc.hide) {
				t.Errorf("cardHead() = %q, want %q and not %q", got, tc.show, tc.hide)
			}
		})
	}
}
