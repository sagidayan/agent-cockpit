package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sagidayan/agent-cockpit/internal/agents"
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
