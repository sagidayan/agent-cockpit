package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
	"github.com/stretchr/testify/require"
)

func TestJoin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		panes     []tmux.Pane
		stored    []state.Agent
		wantKinds map[string]string
		wantStale []string
	}{
		{
			"tracked pane",
			[]tmux.Pane{{ID: "%1", Command: "node"}},
			[]state.Agent{{Pane: "%1", Kind: "opencode", Status: state.Done, Server: "s1"}},
			map[string]string{"%1": "opencode"},
			nil,
		},
		{
			"state from another tmux server is stale",
			[]tmux.Pane{{ID: "%0", Command: "cockpit"}},
			[]state.Agent{{Pane: "%0", Kind: "claude", Server: "old"}},
			map[string]string{},
			[]string{"%0"},
		},
		{
			"other server state on agent pane is rediscovered",
			[]tmux.Pane{{ID: "%0", Command: "claude"}},
			[]state.Agent{{Pane: "%0", Kind: "claude", SessionID: "old", Server: "old"}},
			map[string]string{"%0": "claude"},
			[]string{"%0"},
		},
		{
			"legacy state kept on agent pane",
			[]tmux.Pane{{ID: "%1", Command: "claude"}},
			[]state.Agent{{Pane: "%1", Kind: "claude", Status: state.Done}},
			map[string]string{"%1": "claude"},
			nil,
		},
		{
			"legacy state on non-agent pane is stale",
			[]tmux.Pane{{ID: "%1", Command: "zsh"}},
			[]state.Agent{{Pane: "%1", Kind: "claude"}},
			map[string]string{},
			[]string{"%1"},
		},
		{
			"untracked agent discovered",
			[]tmux.Pane{{ID: "%1", Command: "claude", Cwd: "/repo"}, {ID: "%2", Command: "codex"}},
			nil,
			map[string]string{"%1": "claude", "%2": "codex"},
			nil,
		},
		{
			"non-agent pane ignored",
			[]tmux.Pane{{ID: "%1", Command: "zsh"}},
			nil,
			map[string]string{},
			nil,
		},
		{
			"dead pane is stale",
			[]tmux.Pane{{ID: "%1", Command: "zsh"}},
			[]state.Agent{{Pane: "%9", Kind: "claude"}},
			map[string]string{},
			[]string{"%9"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			live, stale := join(tc.panes, tc.stored, "s1")
			kinds := map[string]string{}
			for _, l := range live {
				require.Equal(t, l.ID, l.Agent.Pane)
				kinds[l.ID] = l.Kind
			}
			require.Equal(t, tc.wantKinds, kinds)
			require.Equal(t, tc.wantStale, stale)
		})
	}
}

func transcriptFile(t *testing.T, at time.Time, end string) string {
	stamp := func(d time.Duration) string { return at.Add(d).UTC().Format(time.RFC3339) }
	lines := []string{
		fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"content":"fix the bug"}}`, stamp(0)),
		fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"content":[{"type":"text","text":"Fixed it."}]}}`, stamp(time.Second)),
	}
	if end != "" {
		lines = append(lines, fmt.Sprintf(end, stamp(2*time.Second)))
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644))
	return path
}

const (
	ended       = `{"type":"system","subtype":"turn_duration","timestamp":%q}`
	interrupted = `{"type":"user","timestamp":%q,"message":{"content":[{"type":"text","text":"[Request interrupted by user]"}]}}`
)

func TestSettled(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		status      state.Status
		end         string
		wantSaved   bool
		wantStatus  state.Status
		wantMessage string
	}{
		{"turn ended without hook", state.Working, ended, true, state.Done, "Fixed it."},
		{"permission prompt answered and turn ended", state.NeedsInput, ended, true, state.Done, "Fixed it."},
		{"interrupted", state.Working, interrupted, true, state.Idle, "Fixed it."},
		{"still working", state.Working, "", false, state.Working, "hook"},
		{"already done", state.Done, ended, false, state.Done, "hook"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prev := state.Agent{Pane: "%1", Kind: "claude", Cwd: "/repo", Status: tc.status, Message: "hook", UpdatedAt: start.Add(-time.Minute), Transcript: transcriptFile(t, start, tc.end)}
			got, saved := settled(prev)
			require.Equal(t, tc.wantSaved, saved)
			require.Equal(t, tc.wantStatus, got.Status)
			require.Equal(t, tc.wantMessage, got.Message)
			require.Equal(t, "/repo", got.Cwd)
		})
	}
}

func TestSettledPersistsOnce(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMUX", "/tmp/sock,1,0")
	path := transcriptFile(t, time.Now().Add(time.Minute), ended)
	require.NoError(t, state.Save(state.Agent{Pane: "%1", Kind: "claude", Status: state.Working, Transcript: path}))

	prev, cur, saved, err := state.Update("%1", settled)
	require.NoError(t, err)
	require.True(t, saved)
	require.Equal(t, state.Working, prev.Status)
	require.Equal(t, state.Done, cur.Status)

	stored, err := state.Load("%1")
	require.NoError(t, err)
	require.Equal(t, state.Done, stored.Status)
	require.Equal(t, "Fixed it.", stored.Message)

	_, _, saved, err = state.Update("%1", settled)
	require.NoError(t, err)
	require.False(t, saved)
}

func costly(id, parent string, cost, closed float64, hasStats bool) Live {
	return Live{
		Agent:    state.Agent{Pane: id, Kind: "claude", ClosedCost: closed},
		Pane:     tmux.Pane{ID: id, Parent: parent},
		Stats:    state.Stats{CostUSD: cost},
		HasStats: hasStats,
	}
}

func TestTreeCost(t *testing.T) {
	t.Parallel()
	live := []Live{
		costly("%1", "", 1, 0.5, true),
		costly("%2", "%1", 2, 0, true),
		costly("%3", "%2", 9, 0.25, false),
		costly("%4", "", 1, 0, true),
	}
	cases := []struct {
		name string
		root int
		want float64
	}{
		{"root with closed and live descendants", 0, 3.75},
		{"middle of the tree", 1, 2.25},
		{"stale stats count only closed cost", 2, 0.25},
		{"separate tree", 3, 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.InDelta(t, tc.want, TreeCost(live, live[tc.root]), 1e-9)
		})
	}
}

func TestWarnBudget(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMUX", "/tmp/sock,1,0")
	live := []Live{
		costly("%1", "", 1, 1, true),
		costly("%2", "%1", 2, 0, true),
		costly("%3", "", 1, 0, true),
	}
	for _, a := range live {
		require.NoError(t, state.Save(a.Agent))
	}
	cfg := config.Default()
	for _, budget := range []struct {
		limit float64
		want  []bool
	}{
		{3, []bool{true, false, false}},
		{10, []bool{false, false, false}},
	} {
		cfg.Budget.TreeUSD = budget.limit
		for i := range live {
			saved, err := state.Load(live[i].ID)
			require.NoError(t, err)
			live[i].Agent = saved
		}
		WarnBudget(live, cfg)
		for i, want := range budget.want {
			require.Equal(t, want, live[i].OverBudget, live[i].ID)
			saved, err := state.Load(live[i].ID)
			require.NoError(t, err)
			require.Equal(t, want, saved.BudgetWarned, live[i].ID)
		}
	}
}

func TestCurrent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		agent state.Agent
		want  state.Status
	}{
		{"working agent", state.Agent{Kind: "codex", Status: state.Working}, state.Working},
		{"paused by cockpit shows paused", state.Agent{Kind: "codex", Status: state.Working, Paused: true}, state.Paused},
		{"paused agent that went idle still shows paused", state.Agent{Kind: "codex", Status: state.Idle, Paused: true}, state.Paused},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Current(tc.agent).Status)
		})
	}
}
