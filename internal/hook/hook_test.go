package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/stretchr/testify/require"
)

const transcriptFixture = `{"type":"user","message":{"role":"user","content":"hi"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"first"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"final answer"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash"}]}}
{"type":"system","subtype":"x"}
`

func TestParse(t *testing.T) {
	t.Parallel()
	transcriptPath := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(transcriptFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		kind  string
		args  []string
		stdin string
		want  event
	}{
		{"claude session start", "claude", nil, `{"hook_event_name":"SessionStart","session_id":"abc","cwd":"/p","source":"startup"}`, event{status: state.Idle, cwd: "/p", session: "abc"}},
		{"claude prompt", "claude", nil, `{"hook_event_name":"UserPromptSubmit","cwd":"/p","prompt":"fix it"}`, event{status: state.Working, prompt: "fix it", cwd: "/p"}},
		{"claude injected prompt", "claude", nil, `{"hook_event_name":"UserPromptSubmit","cwd":"/p","prompt":"<task-notification>done</task-notification>"}`, event{status: state.Working, cwd: "/p"}},
		{"claude notification", "claude", nil, `{"hook_event_name":"Notification","cwd":"/p","message":"Claude needs your permission","notification_type":"permission_prompt"}`, event{status: state.NeedsInput, message: "Claude needs your permission", cwd: "/p"}},
		{"claude stop with last message", "claude", nil, `{"hook_event_name":"Stop","cwd":"/p","last_assistant_message":"all done"}`, event{status: state.Done, message: "all done", cwd: "/p"}},
		{"claude stop with background tasks", "claude", nil, `{"hook_event_name":"Stop","cwd":"/p","last_assistant_message":"started tests","background_tasks":[{"id":"t1","type":"shell","status":"running","description":"go test ./..."},{"id":"t2","type":"subagent","status":"running","description":"review"}]}`, event{status: state.Background, message: "2 in background (go test ./..., review) · started tests", cwd: "/p"}},
		{"claude stop with empty background", "claude", nil, `{"hook_event_name":"Stop","cwd":"/p","last_assistant_message":"all done","background_tasks":[],"session_crons":[]}`, event{status: state.Done, message: "all done", cwd: "/p"}},
		{"claude stop with a scheduled wakeup", "claude", nil, `{"hook_event_name":"Stop","cwd":"/p","last_assistant_message":"waiting","session_crons":[{"id":"c1","schedule":"*/5 * * * *","recurring":true}]}`, event{status: state.Background, message: "scheduled to wake (*/5 * * * *) · waiting", cwd: "/p"}},
		{"claude stop from transcript", "claude", nil, `{"hook_event_name":"Stop","cwd":"/p","transcript_path":"` + transcriptPath + `"}`, event{status: state.Done, message: "final answer", cwd: "/p", transcript: transcriptPath}},
		{"claude session end", "claude", nil, `{"hook_event_name":"SessionEnd","cwd":"/p","reason":"other"}`, event{remove: true, cwd: "/p"}},
		{"claude unknown event", "claude", nil, `{"hook_event_name":"PreToolUse","cwd":"/p"}`, event{cwd: "/p"}},
		{"codex turn complete", "codex", []string{`{"type":"agent-turn-complete","thread-id":"t","turn-id":"1","cwd":"/c","input-messages":["x"],"last-assistant-message":"Rename complete"}`}, "", event{status: state.Done, message: "Rename complete", cwd: "/c", session: "t", prompt: "x"}},
		{"codex other type", "codex", []string{`{"type":"something-else"}`}, "", event{}},
		{"opencode working", "opencode", []string{"working"}, "", event{status: state.Working}},
		{"opencode working with session", "opencode", []string{"session", "ses_1", "working"}, "", event{status: state.Working, session: "ses_1"}},
		{"opencode needs input", "opencode", []string{"needs_input", "bash", "rm -rf x"}, "", event{status: state.NeedsInput, message: "bash rm -rf x"}},
		{"opencode stats", "opencode", []string{"stats", `{"model":"Big Pickle","context_used":7479,"context_max":200000,"cost_usd":0.5}`}, "", event{stats: &state.Stats{Model: "Big Pickle", ContextUsed: 7479, ContextMax: 200000, CostUSD: 0.5}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsers[tc.kind](tc.args, strings.NewReader(tc.stdin))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		kind  string
		args  []string
		stdin string
	}{
		{"claude bad json", "claude", nil, "nope"},
		{"codex no args", "codex", nil, ""},
		{"opencode bad status", "opencode", []string{"busy"}, ""},
		{"opencode no args", "opencode", nil, ""},
		{"opencode session without status", "opencode", []string{"session", "ses_1"}, ""},
		{"opencode bad stats", "opencode", []string{"stats", "nope"}, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := parsers[tc.kind](tc.args, strings.NewReader(tc.stdin)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestMerge(t *testing.T) {
	t.Parallel()
	prev := state.Agent{Kind: "claude", SessionID: "s1", Prompt: "old", Transcript: "/t.jsonl", Touched: []string{"/r"}, ClosedCost: 1.5, BudgetWarned: true, Paused: true}
	resumed := prev
	resumed.Status, resumed.Prompt, resumed.Paused = state.Working, "go on", false
	stillPaused := prev
	stillPaused.Status = state.Working
	cases := []struct {
		name string
		next state.Agent
		want state.Agent
	}{
		{"same session keeps everything", state.Agent{Kind: "claude", SessionID: "s1"}, prev},
		{"new session keeps touched, cost and the pause only", state.Agent{Kind: "claude", SessionID: "s2"}, state.Agent{Kind: "claude", SessionID: "s2", Touched: []string{"/r"}, ClosedCost: 1.5, BudgetWarned: true, Paused: true}},
		{"a new prompt clears a cockpit pause", state.Agent{Kind: "claude", SessionID: "s1", Status: state.Working, Prompt: "go on"}, resumed},
		{"a tool event keeps the pause", state.Agent{Kind: "claude", SessionID: "s1", Status: state.Working}, stillPaused},
		{"other agent kind starts fresh", state.Agent{Kind: "codex"}, state.Agent{Kind: "codex"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, merge(prev, tc.next))
		})
	}
}
