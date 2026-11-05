package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

const (
	prompt      = `{"type":"user","timestamp":"2026-01-01T10:00:00Z","message":{"content":"fix the bug"}}`
	toolResult  = `{"type":"user","timestamp":"2026-01-01T10:00:02Z","message":{"content":[{"type":"tool_result","content":"ok"}]}}`
	reply       = `{"type":"assistant","timestamp":"2026-01-01T10:00:03Z","message":{"content":[{"type":"text","text":"Fixed it."}]}}`
	ended       = `{"type":"system","subtype":"turn_duration","timestamp":"2026-01-01T10:00:04Z"}`
	interrupted = `{"type":"user","timestamp":"2026-01-01T10:00:05Z","message":{"content":[{"type":"text","text":"[Request interrupted by user]"}]}}`
	injected    = `{"type":"user","timestamp":"2026-01-01T10:00:06Z","message":{"content":"<task-notification>x</task-notification>"}}`
	meta        = `{"type":"user","isMeta":true,"timestamp":"2026-01-01T10:00:07Z","message":{"content":"skill text"}}`
)

func at(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, "2026-01-01T"+s+"Z")
	return t
}

func write(t *testing.T, dir, name string, lines ...string) string {
	path := filepath.Join(dir, name+".jsonl")
	data := ""
	for _, l := range lines {
		data += l + "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
	return path
}

func TestRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		lines []string
		want  Turn
	}{
		{"working", []string{prompt, toolResult, reply}, Turn{Prompt: "fix the bug", Reply: "Fixed it.", Started: at("10:00:00")}},
		{"ended", []string{prompt, reply, ended}, Turn{Prompt: "fix the bug", Reply: "Fixed it.", Started: at("10:00:00"), Ended: at("10:00:04")}},
		{"interrupted", []string{prompt, reply, interrupted}, Turn{Prompt: "fix the bug", Reply: "Fixed it.", Started: at("10:00:00"), Ended: at("10:00:05"), Interrupted: true}},
		{"injected prompt starts a turn but keeps the prompt", []string{prompt, reply, ended, injected}, Turn{Prompt: "fix the bug", Started: at("10:00:06")}},
		{"meta entries ignored", []string{prompt, reply, ended, meta}, Turn{Prompt: "fix the bug", Reply: "Fixed it.", Started: at("10:00:00"), Ended: at("10:00:04")}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Read(write(t, t.TempDir(), "s", tc.lines...))
			require.True(t, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestReadFollowsContinuation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := write(t, dir, "old", prompt, `{"type":"continued-in","continuedInSessionId":"new"}`)
	write(t, dir, "new", prompt, reply, ended)
	got, ok := Read(old)
	require.True(t, ok)
	require.Equal(t, at("10:00:04"), got.Ended)
}

func TestSettle(t *testing.T) {
	t.Parallel()
	before, after := at("09:00:00"), at("11:00:00")
	cases := []struct {
		name        string
		lines       []string
		status      state.Status
		updated     time.Time
		wantStatus  state.Status
		wantMessage string
	}{
		{"working turn ended without hook", []string{prompt, reply, ended}, state.Working, before, state.Done, "Fixed it."},
		{"interrupted while working", []string{prompt, reply, interrupted}, state.Working, before, state.Idle, "Fixed it."},
		{"interrupted at a permission prompt", []string{prompt, interrupted}, state.NeedsInput, before, state.Idle, ""},
		{"still working shows progress", []string{prompt, reply}, state.Working, before, state.Working, "Fixed it."},
		{"hook newer than transcript", []string{prompt, reply, ended}, state.Working, after, state.Working, "hook"},
		{"done untouched", []string{prompt, reply, ended}, state.Done, before, state.Done, "hook"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := state.Agent{Kind: "claude", Status: tc.status, Message: "hook", UpdatedAt: tc.updated, Transcript: write(t, t.TempDir(), "s", tc.lines...)}
			got := Settle(a)
			require.Equal(t, tc.wantStatus, got.Status)
			require.Equal(t, tc.wantMessage, got.Message)
			require.Equal(t, "fix the bug", got.Prompt)
		})
	}
}
