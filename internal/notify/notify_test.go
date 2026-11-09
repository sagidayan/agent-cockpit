package notify

import (
	"strings"
	"testing"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

func TestWanted(t *testing.T) {
	t.Parallel()
	cfg := config.Default().Notify
	agent := func(s state.Status, msg string) state.Agent { return state.Agent{Status: s, Message: msg} }
	cases := []struct {
		name      string
		prev, cur state.Agent
		want      bool
	}{
		{"working to done", agent(state.Working, ""), agent(state.Done, "ok"), true},
		{"new agent needs input", state.Agent{}, agent(state.NeedsInput, "perm"), true},
		{"done again with new message", agent(state.Done, "a"), agent(state.Done, "b"), true},
		{"done unchanged", agent(state.Done, "a"), agent(state.Done, "a"), false},
		{"interrupted by a cockpit pause", agent(state.Working, ""), state.Agent{Status: state.Done, Message: "ok", Paused: true}, false},
		{"event not configured", agent(state.Idle, ""), agent(state.Working, "x"), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Wanted(cfg, tc.prev, tc.cur); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompose(t *testing.T) {
	t.Parallel()
	agent := func(s state.Status, msg string) state.Agent {
		return state.Agent{Kind: "claude", Cwd: "/src/payments-service", Status: s, Message: msg}
	}
	cases := []struct {
		name      string
		a         state.Agent
		id        identity
		wantTitle string
		wantBody  string
	}{
		{"label wins", agent(state.Done, "Added retries."), identity{label: "billing bug", window: "claude", project: "payments"}, "✅ billing bug finished", "Added retries.\nClaude · payments"},
		{"project when no label", agent(state.NeedsInput, "Claude needs your permission to use Bash"), identity{window: "claude", project: "payments"}, "🙋 payments needs you", "Claude needs your permission to use Bash\nClaude"},
		{"real window name", agent(state.Done, ""), identity{window: "api-refactor"}, "✅ api-refactor finished", "Claude · payments-service"},
		{"generic window falls back to folder", agent(state.Done, "ok"), identity{window: "node"}, "✅ payments-service finished", "ok\nClaude"},
		{"markdown stripped", agent(state.Done, "## Summary\n- **Fixed** the `retry` bug\n- added tests"), identity{label: "x"}, "✅ x finished", "Summary Fixed the retry bug added tests\nClaude · payments-service"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := compose(tc.a, tc.id)
			if n.title != tc.wantTitle || n.body != tc.wantBody {
				t.Fatalf("got %q / %q, want %q / %q", n.title, n.body, tc.wantTitle, tc.wantBody)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word ", 60)
	got := Summary(long)
	if !strings.HasSuffix(got, "…") || len([]rune(got)) > maxBody+1 || strings.Contains(got, "wor…") {
		t.Fatalf("bad summary %q", got)
	}
}
